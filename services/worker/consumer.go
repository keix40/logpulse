package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

const (
	insertRetryInitial = time.Second
	insertRetryMax     = 30 * time.Second
	pendingClaimCount  = 32
)

// LogStore persists accepted log batches.
type LogStore interface {
	InsertBatch(ctx context.Context, entries []logevent.Entry) error
}

type streamBatch struct {
	entries []logevent.Entry
	ids     []string
}

func (b *streamBatch) append(entry logevent.Entry, id string) {
	b.entries = append(b.entries, entry)
	b.ids = append(b.ids, id)
}

func (b *streamBatch) clear() {
	b.entries = b.entries[:0]
	b.ids = b.ids[:0]
}

func (b *streamBatch) len() int {
	if len(b.ids) > 0 {
		return len(b.ids)
	}
	return len(b.entries)
}

// workerConsumerName returns a unique Redis consumer name for this replica.
func workerConsumerName() string {
	if v := os.Getenv("WORKER_CONSUMER_NAME"); v != "" {
		return v
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return redisx.ConsumerWorker + "-" + host
}

func persistBatch(ctx context.Context, rdb *redis.Client, store LogStore, hub *LiveHub, batch *streamBatch, logger *slog.Logger) error {
	if batch.len() == 0 {
		return nil
	}
	if err := store.InsertBatch(ctx, batch.entries); err != nil {
		return err
	}
	for _, e := range batch.entries {
		hub.Broadcast(e)
	}
	if rdb != nil && len(batch.ids) > 0 {
		if err := rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupWorker, batch.ids...).Err(); err != nil {
			return err
		}
	}
	batch.clear()
	return nil
}

func persistBatchWithRetry(ctx context.Context, rdb *redis.Client, store LogStore, hub *LiveHub, batch *streamBatch, logger *slog.Logger) error {
	backoff := insertRetryInitial
	for batch.len() > 0 {
		if err := persistBatch(ctx, rdb, store, hub, batch, logger); err != nil {
			if logger != nil {
				logger.Error("clickhouse insert failed, will retry", "err", err, "pending", batch.len())
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < insertRetryMax {
				backoff *= 2
				if backoff > insertRetryMax {
					backoff = insertRetryMax
				}
			}
			continue
		}
		return nil
	}
	return nil
}

func flushBatch(ctx context.Context, store LogStore, hub *LiveHub, entries []logevent.Entry, logger *slog.Logger) []logevent.Entry {
	batch := &streamBatch{entries: entries}
	if err := persistBatch(ctx, nil, store, hub, batch, logger); err != nil {
		if logger != nil {
			logger.Error("clickhouse insert failed", "err", err)
		}
		return batch.entries
	}
	return batch.entries
}

func reclaimPendingOnStart(ctx context.Context, rdb *redis.Client, consumer string, handler func([]redis.XMessage), logger *slog.Logger) {
	start := "0-0"
	for {
		msgs, nextStart, err := rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   redisx.StreamLogs,
			Group:    redisx.GroupWorker,
			Consumer: consumer,
			MinIdle:  0,
			Start:    start,
			Count:    pendingClaimCount,
		}).Result()
		if err != nil {
			if logger != nil {
				logger.Warn("xautoclaim", "err", err)
			}
			return
		}
		if len(msgs) > 0 {
			handler(msgs)
		}
		if nextStart == "0-0" || len(msgs) == 0 {
			return
		}
		start = nextStart
	}
}

func processStreamMessages(ctx context.Context, rdb *redis.Client, batch *streamBatch, msgs []redis.XMessage, logger *slog.Logger) {
	for _, msg := range msgs {
		raw, _ := msg.Values[redisx.FieldPayload].(string)
		entry, err := logevent.ParseEntryJSON(raw)
		if err != nil {
			if logger != nil {
				logger.Warn("bad payload", "id", msg.ID)
			}
			_ = rdb.XAck(ctx, redisx.StreamLogs, redisx.GroupWorker, msg.ID)
			continue
		}
		batch.append(entry, msg.ID)
	}
}

func runStreamConsumer(ctx context.Context, rdb *redis.Client, store LogStore, hub *LiveHub, logger *slog.Logger) {
	consumer := workerConsumerName()
	batch := &streamBatch{}
	flush := func() {
		_ = persistBatchWithRetry(ctx, rdb, store, hub, batch, logger)
	}

	reclaimPendingOnStart(ctx, rdb, consumer, func(msgs []redis.XMessage) {
		processStreamMessages(ctx, rdb, batch, msgs, logger)
		if batch.len() >= 64 {
			flush()
		}
	}, logger)
	if batch.len() > 0 {
		flush()
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
			if err != redis.Nil && logger != nil {
				logger.Error("xreadgroup", "err", err)
			}
			continue
		}
		for _, s := range streams {
			processStreamMessages(ctx, rdb, batch, s.Messages, logger)
			if batch.len() >= 64 {
				flush()
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
