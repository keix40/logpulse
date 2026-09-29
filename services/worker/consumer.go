package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

// LogStore persists accepted log batches.
type LogStore interface {
	InsertBatch(ctx context.Context, entries []logevent.Entry) error
}

func flushBatch(ctx context.Context, store LogStore, hub *LiveHub, batch []logevent.Entry, logger *slog.Logger) []logevent.Entry {
	if len(batch) == 0 {
		return batch
	}
	if err := store.InsertBatch(ctx, batch); err != nil {
		if logger != nil {
			logger.Error("clickhouse insert failed", "err", err)
		}
	} else {
		for _, e := range batch {
			hub.Broadcast(e)
		}
	}
	return batch[:0]
}

func runStreamConsumer(ctx context.Context, rdb *redis.Client, store LogStore, hub *LiveHub, logger *slog.Logger) {
	consumer := redisx.ConsumerWorker + "-1"
	batch := make([]logevent.Entry, 0, 64)
	flush := func() {
		batch = flushBatch(ctx, store, hub, batch, logger)
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

func ensureGroup(ctx context.Context, rdb *redis.Client, stream, group string, logger *slog.Logger) {
	err := rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		logger.Warn("consumer group create", "stream", stream, "group", group, "err", err)
	}
}
