package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"recommendation-service/internal/auth"
	"recommendation-service/internal/config"
	"recommendation-service/internal/features"
	"recommendation-service/internal/handler"
	"recommendation-service/internal/kafka"
	"recommendation-service/internal/streamclient"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := features.New(cfg.RedisURL)
	if err != nil {
		slog.Error("connect to redis", "err", err)
		os.Exit(1)
	}
	defer store.Close()
	if err := store.Ping(ctx); err != nil {
		slog.Error("ping redis", "err", err)
		os.Exit(1)
	}

	streams := streamclient.New(cfg.StreamServiceURL)
	h := handler.New(store, streams, cfg)

	consumerCtx, cancelConsumers := context.WithCancel(context.Background())
	defer cancelConsumers()
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "user-events", "recommendation-service-views", h.HandleUserEvent)
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "subscription-events", "recommendation-service-subscriptions", h.HandleSubscriptionEvent)
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "gift-events", "recommendation-service-gifts", h.HandleGiftEvent)
	slog.Info("kafka consumers started", "topics", "user-events,subscription-events,gift-events")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.Handle("GET /feed", auth.RequireAuth(cfg.JWTSecret)(http.HandlerFunc(h.GetFeed)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(logRequests(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("recommendation-service listening", "port", cfg.Port)
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
