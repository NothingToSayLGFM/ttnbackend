package handler

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

type DesktopHandler struct {
	users          *repository.UserRepo
	sessions       *repository.SessionRepo
	desktopAppPath string
	zebraAppPath   string
}

func NewDesktopHandler(users *repository.UserRepo, sessions *repository.SessionRepo, desktopAppPath, zebraAppPath string) *DesktopHandler {
	return &DesktopHandler{users: users, sessions: sessions, desktopAppPath: desktopAppPath, zebraAppPath: zebraAppPath}
}

// DownloadApp generates a desktop token, bundles config.json into the app zip, and serves it.
func (h *DesktopHandler) DownloadApp(w http.ResponseWriter, r *http.Request) {
	userID := mw.GetUserID(r)
	u, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		Error(w, http.StatusNotFound, "user not found")
		return
	}

	token, err := generateToken()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	if err := h.users.SetDesktopToken(r.Context(), userID, token); err != nil {
		Error(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"TTNScanner.zip\"")

	if err := h.buildZipTo(w, u.Email, token, h.desktopAppPath, "TTNScanner"); err != nil {
		// Headers already sent, can't send error JSON — log it
		fmt.Printf("desktop: build zip error: %v\n", err)
	}
}

// DownloadZebraApp serves the Zebra TC26 scanner app with the existing desktop token embedded.
// Does NOT regenerate the token — both apps share the same token.
func (h *DesktopHandler) DownloadZebraApp(w http.ResponseWriter, r *http.Request) {
	userID := mw.GetUserID(r)
	u, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		Error(w, http.StatusNotFound, "user not found")
		return
	}

	// Ensure user has a token; generate one if missing
	token := u.DesktopToken
	if token == "" {
		token, err = generateToken()
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		if err := h.users.SetDesktopToken(r.Context(), userID, token); err != nil {
			Error(w, http.StatusInternalServerError, "failed to save token")
			return
		}
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"ZebraScanner.zip\"")

	if err := h.buildZipTo(w, u.Email, token, h.zebraAppPath, "ZebraScanner"); err != nil {
		fmt.Printf("desktop: build zebra zip error: %v\n", err)
	}
}

// ResetToken regenerates the desktop token without downloading the app.
func (h *DesktopHandler) ResetToken(w http.ResponseWriter, r *http.Request) {
	userID := mw.GetUserID(r)
	token, err := generateToken()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	if err := h.users.SetDesktopToken(r.Context(), userID, token); err != nil {
		Error(w, http.StatusInternalServerError, "failed to save token")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"message": "token reset"})
}

// Balance is a public endpoint (no JWT). Accepts {email, token}, returns scan_balance.
func (h *DesktopHandler) Balance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}
	if err := Decode(r, &body); err != nil || body.Email == "" || body.Token == "" {
		Error(w, http.StatusBadRequest, "email and token required")
		return
	}

	u, err := h.users.FindByEmailAndToken(r.Context(), body.Email, body.Token)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	JSON(w, http.StatusOK, map[string]int{"scan_balance": u.ScanBalance})
}

// Deduct is a public endpoint (no JWT). Accepts {email, token, count}, deducts from balance.
func (h *DesktopHandler) Deduct(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Token string `json:"token"`
		Count int    `json:"count"`
	}
	if err := Decode(r, &body); err != nil || body.Email == "" || body.Token == "" || body.Count <= 0 {
		Error(w, http.StatusBadRequest, "email, token and count required")
		return
	}

	u, err := h.users.FindByEmailAndToken(r.Context(), body.Email, body.Token)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Unlimited balance (admin or special user): skip deduction
	if u.ScanBalance == -1 {
		JSON(w, http.StatusOK, map[string]int{"scan_balance": -1})
		return
	}

	if err := h.users.DeductScanBalance(r.Context(), u.ID, body.Count); err != nil {
		Error(w, http.StatusPaymentRequired, "insufficient balance")
		return
	}

	updated, _ := h.users.FindByID(r.Context(), u.ID)
	JSON(w, http.StatusOK, map[string]int{"scan_balance": updated.ScanBalance})
}

// ScanReport is a public endpoint (no JWT).
// Accepts {email, token, device_type, ttns:[{ttn,status,registry}]},
// saves the session with TTNs, and deducts only 'done' TTNs from balance.
func (h *DesktopHandler) ScanReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email      string `json:"email"`
		Token      string `json:"token"`
		DeviceType string `json:"device_type"`
		TTNs       []struct {
			TTN      string `json:"ttn"`
			Status   string `json:"status"`
			Registry string `json:"registry"`
			Message  string `json:"message"`
		} `json:"ttns"`
	}
	if err := Decode(r, &body); err != nil || body.Email == "" || body.Token == "" {
		Error(w, http.StatusBadRequest, "email, token and ttns required")
		return
	}

	deviceType := body.DeviceType
	if deviceType == "" {
		deviceType = "desktop"
	}

	u, err := h.users.FindByEmailAndToken(r.Context(), body.Email, body.Token)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Build domain TTNs
	ttns := make([]*domain.SessionTTN, 0, len(body.TTNs))
	doneCount := 0
	for _, t := range body.TTNs {
		ttns = append(ttns, &domain.SessionTTN{
			TTN:      t.TTN,
			Status:   t.Status,
			Message:  t.Message,
			Registry: t.Registry,
		})
		if t.Status == "done" {
			doneCount++
		}
	}

	if _, err := h.sessions.CreateFinished(r.Context(), u.ID, deviceType, ttns); err != nil {
		Error(w, http.StatusInternalServerError, "failed to save session")
		return
	}

	// Deduct only 'done' TTNs. If balance < doneCount, deduct whatever remains (partial, down to 0).
	actualDeduct := 0
	if doneCount > 0 && u.ScanBalance != -1 {
		actualDeduct = doneCount
		if u.ScanBalance < doneCount {
			actualDeduct = u.ScanBalance // partial: drain to 0
		}
		if actualDeduct > 0 {
			_ = h.users.DeductScanBalance(r.Context(), u.ID, actualDeduct)
		}
	}

	updated, _ := h.users.FindByID(r.Context(), u.ID)
	balance := -1
	if updated != nil {
		balance = updated.ScanBalance
	}
	JSON(w, http.StatusOK, map[string]any{"scan_balance": balance, "deducted": doneCount})
}

// buildZipTo writes appPath folder + config.json into w as a zip archive.
// folderName is used as the zip root folder name and as the config.json parent path.
func (h *DesktopHandler) buildZipTo(w io.Writer, email, token, appPath, folderName string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	if _, err := os.Stat(appPath); err != nil {
		return fmt.Errorf("app path not found: %s", appPath)
	}

	err := filepath.Walk(appPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(filepath.Dir(appPath), path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		f, err := zw.Create(rel)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(f, src)
		return err
	})
	if err != nil {
		return fmt.Errorf("walking app folder: %w", err)
	}

	cfg := map[string]string{
		"email":         email,
		"desktop_token": token,
	}
	cfgBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	cfgEntry, err := zw.Create(folderName + "/config.json")
	if err != nil {
		return fmt.Errorf("create config entry: %w", err)
	}
	_, err = cfgEntry.Write(cfgBytes)
	return err
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
