package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notification-service/internal/auth"
	"notification-service/internal/authclient"
	"notification-service/internal/config"
	"notification-service/internal/db"
	"notification-service/internal/email"
	"notification-service/internal/handler"
	"notification-service/internal/kafka"
	"notification-service/internal/realtime"
	"notification-service/internal/subscriptionclient"
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

	authC := authclient.New(cfg.AuthServiceURL)
	subsC := subscriptionclient.New(cfg.SubscriptionServiceURL)
	h := handler.New(database, rt, authC, subsC, email.ConsoleSender{}, cfg)

	consumerCtx, cancelConsumers := context.WithCancel(context.Background())
	defer cancelConsumers()
	// each topic gets its own consumer group ID -- one process consuming
	// three unrelated topics is three independent subscriptions, each
	// with its own offset tracking. Reusing one group ID across readers
	// subscribed to different topics confuses Kafka's group coordinator
	// (members of a group are expected to have consistent subscriptions)
	// and can leave a reader stuck with no partition assignment, silently.
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "subscription-events", "notification-service-subscriptions", h.HandleSubscriptionEvent)
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "gift-events", "notification-service-gifts", h.HandleGiftEvent)
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "stream-events", "notification-service-streams", h.HandleStreamEvent)
	slog.Info("kafka consumers started", "topics", "subscription-events,gift-events,stream-events")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)

	authed := auth.RequireAuth(cfg.JWTSecret)
	mux.Handle("GET /notifications", authed(http.HandlerFunc(h.ListNotifications)))
	mux.Handle("POST /notifications/{id}/read", authed(http.HandlerFunc(h.MarkRead)))
	mux.Handle("GET /notifications/ws", auth.RequireAuthWS(cfg.JWTSecret)(http.HandlerFunc(h.NotificationsWS)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(logRequests(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("notification-service listening", "port", cfg.Port)
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
