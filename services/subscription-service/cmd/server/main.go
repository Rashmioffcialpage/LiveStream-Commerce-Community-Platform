package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subscription-service/internal/auth"
	"subscription-service/internal/config"
	"subscription-service/internal/db"
	"subscription-service/internal/handler"
	"subscription-service/internal/kafka"
	"subscription-service/internal/paymentclient"
	"subscription-service/internal/streamclient"
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

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	payments := paymentclient.New(cfg.PaymentServiceURL)
	streams := streamclient.New(cfg.StreamServiceURL)
	h := handler.New(database, producer, payments, streams, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /internal/channels/{id}/subscribers", h.ListSubscribersInternal)

	authed := auth.RequireAuth(cfg.JWTSecret)
	mux.Handle("POST /channels/{slug}/subscribe", authed(http.HandlerFunc(h.Subscribe)))
	mux.Handle("GET /channels/{slug}/subscribers", authed(http.HandlerFunc(h.ListSubscribers)))
	mux.Handle("GET /me/subscriptions", authed(http.HandlerFunc(h.MySubscriptions)))
	mux.Handle("POST /subscriptions/{id}/cancel", authed(http.HandlerFunc(h.CancelSubscription)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(logRequests(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("subscription-service listening", "port", cfg.Port)
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

// see auth-service/cmd/server/main.go's withCORS for the same note.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
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
