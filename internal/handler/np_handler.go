package handler

import (
	"net/http"
	"time"

	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/novaposhta"
	"ttnflow-api/internal/repository"
)

type NPHandler struct {
	client  *novaposhta.Client
	apiKeys *repository.APIKeyRepo
	users   *repository.UserRepo
}

func NewNPHandler(client *novaposhta.Client, apiKeys *repository.APIKeyRepo, users *repository.UserRepo) *NPHandler {
	return &NPHandler{client: client, apiKeys: apiKeys, users: users}
}

func (h *NPHandler) apiKey(r *http.Request) (string, error) {
	k, err := h.apiKeys.FindActiveByUserID(r.Context(), mw.GetUserID(r))
	if err != nil {
		return "", err
	}
	return k.APIKey, nil
}

func (h *NPHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TTNs []string `json:"ttns"`
	}
	if err := Decode(r, &body); err != nil || len(body.TTNs) == 0 {
		Error(w, http.StatusBadRequest, "ttns array required")
		return
	}
	apiKey, err := h.apiKey(r)
	if err != nil || apiKey == "" {
		Error(w, http.StatusBadRequest, "np api key not set in profile")
		return
	}

	// Deduplicate: keep last occurrence
	seen := map[string]int{}
	for i, ttn := range body.TTNs {
		seen[ttn] = i
	}
	unique := make([]string, 0, len(seen))
	dupMark := map[string]bool{}
	for i, ttn := range body.TTNs {
		if seen[ttn] != i {
			dupMark[ttn] = true
		} else {
			unique = append(unique, ttn)
		}
	}

	// No balance check at validation — deduction happens after distribute only

	results := make([]novaposhta.ValidateResult, 0, len(body.TTNs))
	for _, ttn := range body.TTNs {
		if dupMark[ttn] {
			results = append(results, novaposhta.ValidateResult{TTN: ttn, Status: novaposhta.StatusDuplicate, Message: "дублікат"})
		}
	}
	for _, ttn := range unique {
		results = append(results, novaposhta.ValidateTTN(h.client, apiKey, ttn))
	}

	groups := novaposhta.GroupResults(results)
	JSON(w, http.StatusOK, map[string]any{
		"results": results,
		"groups":  groups,
	})
}

func (h *NPHandler) Distribute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionID string                       `json:"session_id"`
		Groups    []novaposhta.DistributeInput `json:"groups"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	apiKey, err := h.apiKey(r)
	if err != nil || apiKey == "" {
		Error(w, http.StatusBadRequest, "np api key not set in profile")
		return
	}

	results := novaposhta.Distribute(h.client, apiKey, body.Groups)

	// Deduct only 'done' TTNs after distribution
	newBalance := -1
	if mw.GetRole(r) != domain.RoleAdmin {
		userID := mw.GetUserID(r)
		doneCount := 0
		for _, res := range results {
			if res.Status == "done" {
				doneCount++
			}
		}
		if doneCount > 0 {
			u, _ := h.users.FindByID(r.Context(), userID)
			if u != nil && u.ScanBalance != -1 {
				toDeduct := doneCount
				if u.ScanBalance < toDeduct {
					toDeduct = u.ScanBalance
				}
				if toDeduct > 0 {
					_ = h.users.DeductScanBalance(r.Context(), userID, toDeduct)
				}
			}
		}
		if updated, err := h.users.FindByID(r.Context(), userID); err == nil {
			newBalance = updated.ScanBalance
		}
	}

	JSON(w, http.StatusOK, map[string]any{"results": results, "scan_balance": newBalance})
}

func (h *NPHandler) ScanSheets(w http.ResponseWriter, r *http.Request) {
	apiKey, err := h.apiKey(r)
	if err != nil || apiKey == "" {
		Error(w, http.StatusBadRequest, "np api key not set")
		return
	}
	sheets, err := h.client.GetScanSheetList(apiKey)
	if err != nil {
		Error(w, http.StatusBadGateway, "nova poshta error")
		return
	}
	JSON(w, http.StatusOK, sheets)
}

func (h *NPHandler) Printed(w http.ResponseWriter, r *http.Request) {
	apiKey, err := h.apiKey(r)
	if err != nil || apiKey == "" {
		Error(w, http.StatusBadRequest, "np api key not set")
		return
	}
	docs, err := h.client.GetPrintedDocuments(apiKey, time.Now())
	if err != nil {
		Error(w, http.StatusBadGateway, "nova poshta error")
		return
	}
	JSON(w, http.StatusOK, docs)
}
