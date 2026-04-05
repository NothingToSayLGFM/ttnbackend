package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

type SubscriptionHandler struct {
	subs *repository.SubscriptionRepo
}

func NewSubscriptionHandler(subs *repository.SubscriptionRepo) *SubscriptionHandler {
	return &SubscriptionHandler{subs: subs}
}

func (h *SubscriptionHandler) Grant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		StartsAt string `json:"starts_at"`
		EndsAt   string `json:"ends_at"`
		Note     string `json:"note"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	startsAt, err := time.Parse("2006-01-02", body.StartsAt)
	if err != nil {
		Error(w, http.StatusBadRequest, "starts_at must be YYYY-MM-DD")
		return
	}
	endsAt, err := time.Parse("2006-01-02", body.EndsAt)
	if err != nil {
		Error(w, http.StatusBadRequest, "ends_at must be YYYY-MM-DD")
		return
	}

	sub := &domain.Subscription{
		UserID:    chi.URLParam(r, "id"),
		GrantedBy: mw.GetUserID(r),
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Note:      body.Note,
	}
	if err := h.subs.Create(r.Context(), sub); err != nil {
		Error(w, http.StatusInternalServerError, "failed to grant subscription")
		return
	}
	JSON(w, http.StatusCreated, sub)
}

func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	subs, err := h.subs.ListByUserID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	JSON(w, http.StatusOK, subs)
}

func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	_ = h.subs.Delete(r.Context(), chi.URLParam(r, "sub_id"))
	JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
