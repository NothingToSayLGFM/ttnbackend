package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

type APIKeyHandler struct {
	keys *repository.APIKeyRepo
}

func NewAPIKeyHandler(keys *repository.APIKeyRepo) *APIKeyHandler {
	return &APIKeyHandler{keys: keys}
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.keys.ListByUserID(r.Context(), mw.GetUserID(r))
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	if keys == nil {
		keys = []*domain.NPAPIKey{}
	}
	JSON(w, http.StatusOK, keys)
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label  string `json:"label"`
		APIKey string `json:"api_key"`
	}
	if err := Decode(r, &body); err != nil || body.APIKey == "" {
		Error(w, http.StatusBadRequest, "label and api_key required")
		return
	}
	if body.Label == "" {
		body.Label = "Ключ"
	}
	k, err := h.keys.Create(r.Context(), mw.GetUserID(r), body.Label, body.APIKey)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusCreated, k)
}

func (h *APIKeyHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.keys.Activate(r.Context(), id, mw.GetUserID(r))
	if errors.Is(err, domain.ErrNotFound) {
		Error(w, http.StatusNotFound, "key not found")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"message": "activated"})
}

func (h *APIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.keys.Delete(r.Context(), id, mw.GetUserID(r)); err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
