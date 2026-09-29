package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/logpulse/logpulse/pkg/alertengine"
	"github.com/logpulse/logpulse/pkg/alerterstream"
	"github.com/logpulse/logpulse/pkg/notify"
	"github.com/logpulse/logpulse/pkg/redisx"
)

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rules, err := alertengine.LoadRules(cfg.RulesPath)
	if err != nil {
		logger.Error("load rules", "err", err)
		os.Exit(1)
	}
	engine := alertengine.NewEngine(rules)
	notifier := notify.New(notifyConfig(cfg), logger)

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
	if err := rdb.XGroupCreateMkStream(ctx, redisx.StreamLogs, redisx.GroupAlerter, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		logger.Warn("consumer group create", "err", err)
	}

	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		logger.Info("alerter health listening", "addr", cfg.HTTPAddr)
		_ = http.ListenAndServe(cfg.HTTPAddr, nil)
	}()

	runCtx, cancel := context.WithCancel(ctx)
	opts := alerterstream.NotifyOptions{MaxAttempts: cfg.MaxNotifyAttempts}
	go alerterstream.Run(runCtx, rdb, engine, notifier, opts, logger)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
	_ = rdb.Close()
}

func notifyConfig(cfg config) notify.Config {
	return notify.Config{
		SlackWebhookURL:   cfg.SlackWebhookURL,
		DiscordWebhookURL: cfg.DiscordWebhookURL,
		TelegramBotToken:  cfg.TelegramBotToken,
		TelegramChatID:    cfg.TelegramChatID,
	}
}
