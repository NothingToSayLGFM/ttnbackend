package middleware

import (
	"net/http"

	"ttnflow-api/internal/domain"
	"ttnflow-api/internal/repository"
)

func RequireSubscription(subs *repository.SubscriptionRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Admin always has access
			if GetRole(r) == domain.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}
			userID := GetUserID(r)
			_, err := subs.FindActiveByUserID(r.Context(), userID)
			if err == domain.ErrNotFound {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				w.Write([]byte(`{"error":"no_subscription","message":"Купіть підписку для продовження"}`))
				return
			}
			if err != nil {
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
