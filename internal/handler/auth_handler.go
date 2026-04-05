package handler

import (
	"errors"
	"net/http"

	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Email == "" || body.Name == "" || body.Password == "" {
		Error(w, http.StatusBadRequest, "email, name, password are required")
		return
	}
	user, err := h.auth.Register(r.Context(), body.Email, body.Name, body.Password)
	if errors.Is(err, domain.ErrConflict) {
		Error(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "registration failed")
		return
	}
	JSON(w, http.StatusCreated, user.Public())
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, pair, err := h.auth.Login(r.Context(), body.Email, body.Password)
	if errors.Is(err, domain.ErrUnauthorized) {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "login failed")
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"user":          user.Public(),
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := Decode(r, &body); err != nil || body.RefreshToken == "" {
		Error(w, http.StatusBadRequest, "refresh_token required")
		return
	}
	pair, err := h.auth.Refresh(r.Context(), body.RefreshToken)
	if errors.Is(err, domain.ErrUnauthorized) {
		Error(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "refresh failed")
		return
	}
	JSON(w, http.StatusOK, pair)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = Decode(r, &body)
	if body.RefreshToken != "" {
		_ = h.auth.Logout(r.Context(), body.RefreshToken)
	}
	_ = mw.GetUserID(r) // just ensure middleware ran
	JSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}
