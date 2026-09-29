package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/logpulse/logpulse/pkg/auth"
	"github.com/logpulse/logpulse/pkg/ingestapi"
	"github.com/logpulse/logpulse/pkg/redisx"
)

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rdb, err := redisx.NewClientFromEnv()
	if err != nil {
		logger.Error("redis config", "err", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping failed", "err", err)
		os.Exit(1)
	}
	cancel()

	publisher := &ingestapi.StreamPublisher{Client: rdb, Logger: logger, StreamMaxLen: cfg.StreamMaxLen}
	limits := limitsFromConfig(cfg)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(30*time.Second))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/v1/logs", auth.RequireBearerFunc(cfg.IngestAPIKey, ingestapi.HandleBatch(publisher, limits, logger)))

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
		go ingestapi.ServeSyslog(syslogLn, publisher, logger)
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
