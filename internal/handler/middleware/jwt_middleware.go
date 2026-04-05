package middleware

import (
	"context"
	"net/http"
	"strings"

	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/service"
)

type contextKey string

const (
	CtxUserID contextKey = "userID"
	CtxRole   contextKey = "role"
)

func JWT(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			userID, role, err := auth.ValidateAccessToken(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), CtxUserID, userID)
			ctx = context.WithValue(ctx, CtxRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(CtxRole).(domain.Role)
		if role != domain.RoleAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GetUserID(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserID).(string)
	return v
}

func GetRole(r *http.Request) domain.Role {
	v, _ := r.Context().Value(CtxRole).(domain.Role)
	return v
}
