package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/logpulse/logpulse/pkg/auth"
	"github.com/logpulse/logpulse/pkg/livehub"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/logpulse/logpulse/pkg/storage"
	"github.com/logpulse/logpulse/pkg/workerstream"
)

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rdb, err := redisx.NewClientFromEnv()
	if err != nil {
		logger.Error("redis config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping failed", "err", err)
		os.Exit(1)
	}
	workerstream.EnsureGroup(ctx, rdb, redisx.StreamLogs, redisx.GroupWorker, logger)

	store, err := storage.OpenFromEnv()
	if err != nil {
		logger.Error("storage open failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	hub := livehub.New()

	workerCtx, workerCancel := context.WithCancel(ctx)
	go workerstream.RunConsumer(workerCtx, rdb, store, hub, logger)

	retentionCtx, retentionCancel := context.WithCancel(ctx)
	defer retentionCancel()
	storage.StartRetentionLoop(retentionCtx, store, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization"},
		AllowCredentials: true,
	}))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	readAuth := func(next http.Handler) http.Handler {
		return auth.RequireBearer(cfg.ReadAPIKey, next)
	}
	r.With(readAuth).Get("/v1/live", hub.SSEHandler())
	r.With(readAuth).Get("/v1/logs/search", store.SearchHandler(logger))

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		logger.Info("worker http listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http error", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = rdb.Close()
}

type config struct {
	HTTPAddr    string
	CORSOrigins []string
	ReadAPIKey  string
}

func loadConfig() config {
	origins := envOr("WORKER_CORS_ORIGINS", "http://localhost:3000")
	return config{
		HTTPAddr:    envOr("WORKER_HTTP_ADDR", "0.0.0.0:8081"),
		CORSOrigins: strings.Split(origins, ","),
		ReadAPIKey:  os.Getenv("READ_API_KEY"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
