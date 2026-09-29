package workerstream

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/logpulse/logpulse/pkg/livehub"
	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

type mockStore struct {
	batches  [][]logevent.Entry
	err      error
	failures atomic.Int32
	failN    int32
}

func (m *mockStore) InsertBatch(_ context.Context, entries []logevent.Entry) error {
	if m.failN > 0 && m.failures.Add(1) <= m.failN {
		return errors.New("store down")
	}
	if m.err != nil {
		return m.err
	}
	cp := append([]logevent.Entry(nil), entries...)
	m.batches = append(m.batches, cp)
	return nil
}

func (m *mockStore) SearchHandler(_ *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {}
}

func (m *mockStore) Close() error { return nil }

func TestFlushBatchPersistsAndFansOut(t *testing.T) {
	store := &mockStore{}
	hub := livehub.New()
	ch := make(chan []byte, 4)
	hub.SubscribeTestChannel(ch)

	entries := []logevent.Entry{{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelInfo,
		Service:   "e2e",
		Message:   "hello",
	}}

	rest := FlushBatch(context.Background(), store, hub, entries, nil)
	if len(rest) != 0 {
		t.Fatalf("expected empty batch after flush")
	}
	if len(store.batches) != 1 || len(store.batches[0]) != 1 {
		t.Fatalf("store did not receive batch")
	}

	select {
	case raw := <-ch:
		var got logevent.Entry
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.Message != "hello" {
			t.Fatalf("broadcast message = %q", got.Message)
		}
	default:
		t.Fatal("expected live broadcast")
	}
}

func TestFlushBatchSkipsFanOutOnStoreError(t *testing.T) {
	store := &mockStore{err: errors.New("store down")}
	hub := livehub.New()
	ch := make(chan []byte, 1)
	hub.SubscribeTestChannel(ch)

	entries := []logevent.Entry{{
		Level: logevent.LevelInfo, Service: "s", Message: "m",
	}}
	rest := FlushBatch(context.Background(), store, hub, entries, nil)
	if len(rest) != 1 {
		t.Fatalf("expected batch retained on store error, got len=%d", len(rest))
	}
	select {
	case <-ch:
		t.Fatal("should not broadcast when store fails")
	default:
	}
}

func TestFailedInsertDoesNotAckMessages(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, redisx.StreamLogs, redisx.GroupWorker, "0").Err(); err != nil {
		t.Fatal(err)
	}

	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelInfo,
		Service:   "worker-test",
		Message:   "pending until insert succeeds",
	}
	raw, err := entry.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	msgID, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: redisx.StreamLogs,
		Values: map[string]interface{}{redisx.FieldPayload: raw},
	}).Result()
	if err != nil {
		t.Fatal(err)
	}

	streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    redisx.GroupWorker,
		Consumer: "worker-dead",
		Streams:  []string{redisx.StreamLogs, ">"},
		Count:    1,
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatal("expected one stream message")
	}
	if streams[0].Messages[0].ID != msgID {
		t.Fatalf("message id = %q want %q", streams[0].Messages[0].ID, msgID)
	}

	pendingBefore, err := rdb.XPending(ctx, redisx.StreamLogs, redisx.GroupWorker).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pendingBefore.Count != 1 {
		t.Fatalf("pending before = %d want 1", pendingBefore.Count)
	}

	store := &mockStore{failN: 1}
	hub := livehub.New()
	batch := &streamBatch{}
	batch.append(entry, msgID)

	if err := persistBatch(ctx, rdb, store, hub, batch, nil); err == nil {
		t.Fatal("expected insert failure")
	}
	if batch.len() != 1 {
		t.Fatalf("batch len = %d want 1 after failed insert", batch.len())
	}

	pendingMid, err := rdb.XPending(ctx, redisx.StreamLogs, redisx.GroupWorker).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pendingMid.Count != 1 {
		t.Fatalf("pending after failed insert = %d want 1 (message must not be acked)", pendingMid.Count)
	}

	if err := persistBatch(ctx, rdb, store, hub, batch, nil); err != nil {
		t.Fatal(err)
	}
	if batch.len() != 0 {
		t.Fatalf("batch len = %d want 0 after successful insert+ack", batch.len())
	}

	pendingAfter, err := rdb.XPending(ctx, redisx.StreamLogs, redisx.GroupWorker).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pendingAfter.Count != 0 {
		t.Fatalf("pending after ack = %d want 0", pendingAfter.Count)
	}
	if len(store.batches) != 1 || len(store.batches[0]) != 1 {
		t.Fatalf("store batches = %d want one persisted batch", len(store.batches))
	}
}

func TestWorkerConsumerNameUsesHostname(t *testing.T) {
	t.Setenv("WORKER_CONSUMER_NAME", "")
	name := WorkerConsumerName()
	if name == redisx.ConsumerWorker+"-1" || name == redisx.ConsumerWorker {
		t.Fatalf("expected hostname-suffixed consumer, got %q", name)
	}
	if len(name) < len(redisx.ConsumerWorker)+2 {
		t.Fatalf("consumer name = %q", name)
	}
}
