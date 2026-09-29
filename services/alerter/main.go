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

	"github.com/logpulse/logpulse/pkg/alertengine"
	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
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
	notifier := NewNotifier(cfg, logger)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
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
	go consumeAlerts(runCtx, rdb, engine, notifier, logger)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
	_ = rdb.Close()
}

func consumeAlerts(ctx context.Context, rdb *redis.Client, engine *alertengine.Engine, n *Notifier, logger *slog.Logger) {
	consumer := redisx.ConsumerAlerter + "-1"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    redisx.GroupAlerter,
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
		now := time.Now().UTC()
		for _, s := range streams {
			for _, msg := range s.Messages {
				raw, _ := msg.Values[redisx.FieldPayload].(string)
				entry, err := logevent.ParseEntryJSON(raw)
				if err != nil {
					_ = rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupAlerter, msg.ID)
					continue
				}
				for _, inc := range engine.Process(entry, now) {
					if err := n.Notify(ctx, inc); err != nil {
						logger.Error("notify failed", "rule", inc.RuleID, "err", err)
					} else {
						logger.Info("alert sent", "rule", inc.RuleID, "fingerprint", inc.Fingerprint)
					}
				}
				_ = rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupAlerter, msg.ID)
			}
		}
	}
}
