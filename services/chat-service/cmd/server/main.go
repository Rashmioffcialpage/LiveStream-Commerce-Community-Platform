package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chat-service/internal/auth"
	"chat-service/internal/config"
	"chat-service/internal/db"
	"chat-service/internal/handler"
	"chat-service/internal/kafka"
	"chat-service/internal/model"
	"chat-service/internal/realtime"
	"chat-service/internal/streamclient"
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

	rt, err := realtime.New(cfg.RedisURL)
	if err != nil {
		slog.Error("connect to redis", "err", err)
		os.Exit(1)
	}
	defer rt.Close()
	if err := rt.Ping(ctx); err != nil {
		slog.Error("ping redis", "err", err)
		os.Exit(1)
	}

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "chat-service", func(ctx context.Context, m model.ChatMessage) error {
		return database.InsertMessage(ctx, m)
	})
	slog.Info("kafka consumer started", "topic", kafka.Topic)

	streams := streamclient.New(cfg.StreamServiceURL)
	h := handler.New(database, rt, producer, streams, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /streams/{id}/chat/history", h.History)

	authed := auth.RequireAuth(cfg.JWTSecret)
	mux.Handle("GET /streams/{id}/chat", auth.RequireAuthWS(cfg.JWTSecret)(http.HandlerFunc(h.ChatWS)))
	mux.Handle("POST /streams/{id}/chat/mute", authed(http.HandlerFunc(h.Mute)))
	mux.Handle("POST /streams/{id}/chat/unmute", authed(http.HandlerFunc(h.Unmute)))
	mux.Handle("DELETE /messages/{id}", authed(http.HandlerFunc(h.DeleteMessage)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("chat-service listening", "port", cfg.Port)
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

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
