package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

type batchPublisher interface {
	Publish(ctx context.Context, entries []logevent.Entry) error
}

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping failed", "err", err)
		os.Exit(1)
	}
	cancel()

	publisher := &StreamPublisher{client: rdb, logger: logger, streamMaxLen: cfg.StreamMaxLen}
	limits := limitsFromConfig(cfg)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(30*time.Second))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/v1/logs", handleBatch(publisher, limits, logger))

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		logger.Info("ingest http listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	var syslogLn net.Listener
	if cfg.SyslogAddr != "" {
		ln, err := net.Listen("tcp", cfg.SyslogAddr)
		if err != nil {
			logger.Error("syslog listen failed", "err", err)
			os.Exit(1)
		}
		syslogLn = ln
		go serveSyslog(syslogLn, publisher, logger)
		logger.Info("syslog listening", "addr", cfg.SyslogAddr)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	if syslogLn != nil {
		_ = syslogLn.Close()
	}
	_ = rdb.Close()
}

type StreamPublisher struct {
	client       *redis.Client
	logger       *slog.Logger
	streamMaxLen int64
}

func (p *StreamPublisher) Publish(ctx context.Context, entries []logevent.Entry) error {
	pipe := p.client.Pipeline()
	for _, e := range entries {
		payload, err := e.ToJSON()
		if err != nil {
			return err
		}
		pipe.XAdd(ctx, redisx.LogStreamAddArgs(payload, p.streamMaxLen))
	}
	_, err := pipe.Exec(ctx)
	return err
}

func handleBatch(p batchPublisher, limits ingestLimits, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limits.MaxBodyBytes)
		var req logevent.BatchRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		logs, err := logevent.ValidateBatch(req.Logs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := p.Publish(r.Context(), logs); err != nil {
			logger.Error("publish failed", "err", err)
			http.Error(w, "failed to enqueue", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}
}
