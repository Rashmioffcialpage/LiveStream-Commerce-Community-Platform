package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/internal/auth"
	"auth-service/internal/config"
	"auth-service/internal/db"
	"auth-service/internal/handler"
	"auth-service/internal/model"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		slog.Error("run migrations", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	h := handler.New(database, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)

	mux.HandleFunc("POST /signup", h.Signup)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /refresh", h.Refresh)

	mux.HandleFunc("GET /oauth/google/login", h.GoogleLogin)
	mux.HandleFunc("GET /oauth/google/callback", h.GoogleCallback)

	mux.Handle("GET /me", auth.RequireAuth(cfg.JWTSecret)(http.HandlerFunc(h.Me)))

	creatorOnly := auth.RequireAuth(cfg.JWTSecret)(auth.RequireRole(model.RoleCreator)(http.HandlerFunc(h.CreatorOnlyPing)))
	mux.Handle("GET /creator/ping", creatorOnly)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(logRequests(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("auth-service listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// withCORS allows the Next.js dev server (a different origin) to call this
// API directly from the browser. Wide open (any origin) because this is a
// local dev/demo deployment with no cookie-based auth to protect -- a real
// deployment would scope Access-Control-Allow-Origin to the actual
// frontend domain instead of "*".
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
