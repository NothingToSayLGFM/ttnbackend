package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"ttnflow-api/internal/config"
	"ttnflow-api/internal/db"
	"ttnflow-api/internal/handler"
	mw "ttnflow-api/internal/handler/middleware"
	"ttnflow-api/internal/novaposhta"
	"ttnflow-api/internal/repository"
	"ttnflow-api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations ok")

	// Repositories
	userRepo := repository.NewUserRepo(pool)
	tokenRepo := repository.NewTokenRepo(pool)
	subRepo := repository.NewSubscriptionRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)

	// Services
	authSvc := service.NewAuthService(userRepo, tokenRepo, cfg.JWTSecret)

	// NP Client
	npClient := novaposhta.NewClient(cfg.NPAPIURL)

	// Handlers
	authH := handler.NewAuthHandler(authSvc)
	userH := handler.NewUserHandler(userRepo, subRepo)
	subH := handler.NewSubscriptionHandler(subRepo)
	npH := handler.NewNPHandler(npClient, userRepo)
	sessionH := handler.NewSessionHandler(sessionRepo)

	// Middleware factories
	jwtMW := mw.JWT(authSvc)
	requireSub := mw.RequireSubscription(subRepo)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(corsMiddleware)

	r.Route("/api/v1", func(r chi.Router) {
		// Public
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(jwtMW)

			r.Post("/auth/logout", authH.Logout)

			r.Get("/me", userH.Me)
			r.Patch("/me", userH.UpdateMe)
			r.Put("/me/api-key", userH.UpdateAPIKey)
			r.Get("/me/subscription", userH.MySubscription)

			// Sessions (user own)
			r.Get("/sessions", sessionH.List)
			r.Get("/sessions/{id}", sessionH.Get)
			r.Group(func(r chi.Router) {
				r.Use(requireSub)
				r.Post("/sessions", sessionH.Create)
				r.Patch("/sessions/{id}", sessionH.Finish)
			})

			// Nova Poshta proxy (requires subscription)
			r.Group(func(r chi.Router) {
				r.Use(requireSub)
				r.Post("/np/validate", npH.Validate)
				r.Post("/np/distribute", npH.Distribute)
				r.Get("/np/scan-sheets", npH.ScanSheets)
				r.Get("/np/printed", npH.Printed)
			})

			// Admin
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAdmin)
				r.Get("/admin/users", userH.AdminListUsers)
				r.Get("/admin/users/{id}", userH.AdminGetUser)
				r.Patch("/admin/users/{id}", userH.AdminUpdateUser)
				r.Delete("/admin/users/{id}", userH.AdminDeleteUser)
				r.Get("/admin/users/{id}/subscriptions", subH.List)
				r.Post("/admin/users/{id}/subscriptions", subH.Grant)
				r.Delete("/admin/subscriptions/{sub_id}", subH.Delete)
				r.Get("/admin/sessions", sessionH.AdminList)
			})
		})
	})

	log.Printf("starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
