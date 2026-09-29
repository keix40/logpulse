package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping failed", "err", err)
		os.Exit(1)
	}
	ensureGroup(ctx, rdb, redisx.StreamLogs, redisx.GroupWorker, logger)

	store, err := NewClickHouseStore(cfg.ClickHouseDSN)
	if err != nil {
		logger.Error("clickhouse connect failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	hub := NewLiveHub()

	workerCtx, workerCancel := context.WithCancel(ctx)
	go runStreamConsumer(workerCtx, rdb, store, hub, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept"},
		AllowCredentials: true,
	}))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/v1/live", hub.SSEHandler())
	r.Get("/v1/logs/search", store.SearchHandler(logger))

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
