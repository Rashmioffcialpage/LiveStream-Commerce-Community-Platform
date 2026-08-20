package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stream-service/internal/auth"
	"stream-service/internal/config"
	"stream-service/internal/db"
	"stream-service/internal/handler"
	"stream-service/internal/realtime"
	"stream-service/internal/signaling"
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

	presence, err := realtime.NewPresence(cfg.RedisURL)
	if err != nil {
		slog.Error("connect to redis", "err", err)
		os.Exit(1)
	}
	defer presence.Close()
	if err := presence.Ping(ctx); err != nil {
		slog.Error("ping redis", "err", err)
		os.Exit(1)
	}

	hub := signaling.NewHub()
	h := handler.New(database, presence, hub, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /demo", h.Demo)

	mux.HandleFunc("GET /channels", h.ListChannels)
	mux.HandleFunc("GET /channels/{slug}", h.GetChannel)
	mux.HandleFunc("GET /channels/{slug}/streams", h.ListChannelStreams)

	creatorAuth := func(next http.Handler) http.Handler {
		return auth.RequireAuth(cfg.JWTSecret)(auth.RequireRole(auth.RoleCreator)(next))
	}
	mux.Handle("POST /channels", creatorAuth(http.HandlerFunc(h.CreateChannel)))
	mux.Handle("POST /channels/{slug}/streams", creatorAuth(http.HandlerFunc(h.CreateStream)))
	mux.Handle("POST /streams/{id}/go-live", creatorAuth(http.HandlerFunc(h.GoLive)))
	mux.Handle("POST /streams/{id}/end", creatorAuth(http.HandlerFunc(h.EndStream)))

	mux.HandleFunc("GET /streams/{id}", h.GetStream)
	// auth for /signal is handled inside the handler itself: the
	// broadcaster leg needs a query-string token (WebSocket upgrades can't
	// carry an Authorization header from a browser) while the viewer leg
	// is intentionally open, so neither fits the standard middleware shape.
	mux.HandleFunc("GET /streams/{id}/signal", h.Signal)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("stream-service listening", "port", cfg.Port)
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
