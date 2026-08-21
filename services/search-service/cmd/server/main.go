package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"search-service/internal/authclient"
	"search-service/internal/config"
	"search-service/internal/handler"
	"search-service/internal/kafka"
	"search-service/internal/search"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	searchClient, err := search.New(cfg.OpenSearchURL)
	if err != nil {
		slog.Error("configure opensearch client", "err", err)
		os.Exit(1)
	}
	if err := searchClient.Ping(ctx); err != nil {
		slog.Error("ping opensearch", "err", err)
		os.Exit(1)
	}
	if err := searchClient.EnsureIndex(ctx); err != nil {
		slog.Error("ensure channels index", "err", err)
		os.Exit(1)
	}
	slog.Info("opensearch index ready")

	authC := authclient.New(cfg.AuthServiceURL)
	h := handler.New(searchClient, authC, cfg)

	consumerCtx, cancelConsumers := context.WithCancel(context.Background())
	defer cancelConsumers()
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "channel-events", "search-service-channels", h.HandleChannelEvent)
	go kafka.RunConsumer(consumerCtx, cfg.KafkaBrokers, "stream-events", "search-service-streams", h.HandleStreamEvent)
	slog.Info("kafka consumers started", "topics", "channel-events,stream-events")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /search", h.SearchChannels)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(logRequests(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("search-service listening", "port", cfg.Port)
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
