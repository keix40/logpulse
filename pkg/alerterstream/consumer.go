package alerterstream

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/logpulse/logpulse/pkg/alertengine"
	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/notify"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

const (
	defaultNotifyAttempts = 5
	notifyRetryInitial    = time.Second
	notifyRetryMax        = 30 * time.Second
)

type NotifyOptions struct {
	MaxAttempts int
}

type incidentNotifier interface {
	Notify(ctx context.Context, inc alertengine.Incident) error
}

func ConsumerName() string {
	if v := os.Getenv("ALERTER_CONSUMER_NAME"); v != "" {
		return v
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return redisx.ConsumerAlerter + "-" + host
}

func notifyIncidents(ctx context.Context, n incidentNotifier, incidents []alertengine.Incident) error {
	for _, inc := range incidents {
		if err := n.Notify(ctx, inc); err != nil {
			return err
		}
	}
	return nil
}

func deadLetterAlert(ctx context.Context, rdb *redis.Client, msg redis.XMessage, reason string) error {
	values := map[string]interface{}{
		redisx.FieldSourceID: msg.ID,
		redisx.FieldError:    reason,
	}
	if raw, ok := msg.Values[redisx.FieldPayload].(string); ok {
		values[redisx.FieldPayload] = raw
	}
	return rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: redisx.StreamAlerterDLQ,
		Values: values,
	}).Err()
}

// HandleAlertMessage processes one stream message (exported for tests).
func HandleAlertMessage(
	ctx context.Context,
	rdb *redis.Client,
	engine *alertengine.Engine,
	n incidentNotifier,
	msg redis.XMessage,
	opts NotifyOptions,
	logger *slog.Logger,
) error {
	raw, _ := msg.Values[redisx.FieldPayload].(string)
	entry, err := logevent.ParseEntryJSON(raw)
	if err != nil {
		_ = rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupAlerter, msg.ID)
		return nil
	}

	incidents := engine.Process(entry, time.Now().UTC())
	if len(incidents) == 0 {
		return rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupAlerter, msg.ID).Err()
	}

	backoff := notifyRetryInitial
	var lastErr error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		lastErr = notifyIncidents(ctx, n, incidents)
		if lastErr == nil {
			if logger != nil {
				for _, inc := range incidents {
					logger.Info("alert sent", "rule", inc.RuleID, "fingerprint", inc.Fingerprint)
				}
			}
			return rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupAlerter, msg.ID).Err()
		}
		if logger != nil {
			logger.Error("notify failed", "id", msg.ID, "attempt", attempt, "err", lastErr)
		}
		if attempt >= opts.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < notifyRetryMax {
			backoff *= 2
			if backoff > notifyRetryMax {
				backoff = notifyRetryMax
			}
		}
	}

	if err := deadLetterAlert(ctx, rdb, msg, lastErr.Error()); err != nil && logger != nil {
		logger.Error("dead letter failed", "id", msg.ID, "err", err)
		return err
	}
	if logger != nil {
		logger.Warn("alert dead-lettered", "id", msg.ID, "attempts", opts.MaxAttempts)
	}
	return rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupAlerter, msg.ID).Err()
}

// Run consumes the logs stream and sends notifications for matching rules.
func Run(ctx context.Context, rdb *redis.Client, engine *alertengine.Engine, n *notify.Notifier, opts NotifyOptions, logger *slog.Logger) {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultNotifyAttempts
	}
	consumer := ConsumerName()
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
			if err != redis.Nil && !errors.Is(err, context.Canceled) && logger != nil {
				logger.Error("xreadgroup", "err", err)
			}
			continue
		}
		for _, s := range streams {
			for _, msg := range s.Messages {
				if err := HandleAlertMessage(ctx, rdb, engine, n, msg, opts, logger); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					if logger != nil {
						logger.Error("handle alert message", "id", msg.ID, "err", err)
					}
				}
			}
		}
	}
}
