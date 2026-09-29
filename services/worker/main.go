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
	"github.com/logpulse/logpulse/pkg/logevent"
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
	r.Get("/v1/logs/search", store.SearchHandler())

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

func ensureGroup(ctx context.Context, rdb *redis.Client, stream, group string, logger *slog.Logger) {
	err := rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		logger.Warn("consumer group create", "stream", stream, "group", group, "err", err)
	}
}

func runStreamConsumer(ctx context.Context, rdb *redis.Client, store *ClickHouseStore, hub *LiveHub, logger *slog.Logger) {
	consumer := redisx.ConsumerWorker + "-1"
	batch := make([]logevent.Entry, 0, 64)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := store.InsertBatch(ctx, batch); err != nil {
			logger.Error("clickhouse insert failed", "err", err)
		} else {
			for _, e := range batch {
				hub.Broadcast(e)
			}
		}
		batch = batch[:0]
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-ticker.C:
			flush()
		default:
		}

		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    redisx.GroupWorker,
			Consumer: consumer,
			Streams:  []string{redisx.StreamLogs, ">"},
			Count:    32,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if err != redis.Nil {
				logger.Error("xreadgroup", "err", err)
			}
			continue
		}
		for _, s := range streams {
			for _, msg := range s.Messages {
				raw, _ := msg.Values[redisx.FieldPayload].(string)
				entry, err := logevent.ParseEntryJSON(raw)
				if err != nil {
					logger.Warn("bad payload", "id", msg.ID)
					_ = rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupWorker, msg.ID)
					continue
				}
				batch = append(batch, entry)
				_ = rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupWorker, msg.ID)
				if len(batch) >= 64 {
					flush()
				}
			}
		}
	}
}
