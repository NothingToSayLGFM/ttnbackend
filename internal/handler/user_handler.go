package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

type UserHandler struct {
	users *repository.UserRepo
}

func NewUserHandler(users *repository.UserRepo) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.FindByID(r.Context(), mw.GetUserID(r))
	if err != nil {
		Error(w, http.StatusNotFound, "user not found")
		return
	}
	JSON(w, http.StatusOK, u.Public())
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	userID := mw.GetUserID(r)
	if body.Name != "" {
		_ = h.users.UpdateName(r.Context(), userID, body.Name)
	}
	if body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		_ = h.users.UpdatePasswordHash(r.Context(), userID, string(hash))
	}
	u, _ := h.users.FindByID(r.Context(), userID)
	JSON(w, http.StatusOK, u.Public())
}

// Admin handlers

func (h *UserHandler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	users, total, err := h.users.List(r.Context(), limit, offset)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	public := make([]domain.UserPublic, 0, len(users))
	for _, u := range users {
		public = append(public, u.Public())
	}
	JSON(w, http.StatusOK, map[string]any{"data": public, "total": total})
}

func (h *UserHandler) AdminGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.FindByID(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, domain.ErrNotFound) {
		Error(w, http.StatusNotFound, "user not found")
		return
	}
	JSON(w, http.StatusOK, u.Public())
}

func (h *UserHandler) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string      `json:"name"`
		Role domain.Role `json:"role"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	id := chi.URLParam(r, "id")
	if body.Name != "" {
		_ = h.users.UpdateName(r.Context(), id, body.Name)
	}
	if body.Role != "" {
		_ = h.users.UpdateRole(r.Context(), id, body.Role)
	}
	u, _ := h.users.FindByID(r.Context(), id)
	JSON(w, http.StatusOK, u.Public())
}

func (h *UserHandler) AdminSetScanBalance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Balance int `json:"balance"`
	}
	if err := Decode(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.users.UpdateScanBalance(r.Context(), id, body.Balance); err != nil {
		Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	u, _ := h.users.FindByID(r.Context(), id)
	JSON(w, http.StatusOK, u.Public())
}

func (h *UserHandler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	_ = h.users.Delete(r.Context(), chi.URLParam(r, "id"))
	JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
