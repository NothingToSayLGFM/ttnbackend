package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

type SessionHandler struct {
	sessions *repository.SessionRepo
}

func NewSessionHandler(sessions *repository.SessionRepo) *SessionHandler {
	return &SessionHandler{sessions: sessions}
}

func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := mw.GetUserID(r)
	// Abandon any lingering running sessions before starting a new one
	_ = h.sessions.AbandonRunning(r.Context(), userID)
	s, err := h.sessions.Create(r.Context(), userID, "web")
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	JSON(w, http.StatusCreated, s)
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	sessions, total, err := h.sessions.ListByUserID(r.Context(), mw.GetUserID(r), limit, offset)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"data": sessions, "total": total})
}

func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.sessions.FindByID(r.Context(), id)
	if err == domain.ErrNotFound {
		Error(w, http.StatusNotFound, "session not found")
		return
	}
	// ownership check (admin bypasses)
	if mw.GetRole(r) != domain.RoleAdmin && s.UserID != mw.GetUserID(r) {
		Error(w, http.StatusForbidden, "forbidden")
		return
	}
	ttns, _ := h.sessions.ListTTNs(r.Context(), id)
	JSON(w, http.StatusOK, map[string]any{"session": s, "ttns": ttns})
}

func (h *SessionHandler) SaveTTNs(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TTNs []*domain.SessionTTN `json:"ttns"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	id := chi.URLParam(r, "id")
	if len(body.TTNs) > 0 {
		_ = h.sessions.ReplaceTTNs(r.Context(), id, body.TTNs)
	}
	JSON(w, http.StatusOK, map[string]string{"message": "saved"})
}

func (h *SessionHandler) Finish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string               `json:"status"`
		TTNs   []*domain.SessionTTN `json:"ttns"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	id := chi.URLParam(r, "id")
	if len(body.TTNs) > 0 {
		_ = h.sessions.ReplaceTTNs(r.Context(), id, body.TTNs)
	}
	status := domain.SessionDone
	if body.Status == string(domain.SessionError) {
		status = domain.SessionError
	}
	_ = h.sessions.Finish(r.Context(), id, status)
	s, _ := h.sessions.FindByID(r.Context(), id)
	JSON(w, http.StatusOK, s)
}

func (h *SessionHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	sessions, total, err := h.sessions.ListAll(r.Context(), limit, offset)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"data": sessions, "total": total})
}
