package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/logpulse/logpulse/pkg/alertengine"
	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

type stubNotifier struct {
	calls atomic.Int32
	failN int32
}

func (s *stubNotifier) Notify(_ context.Context, _ alertengine.Incident) error {
	n := s.calls.Add(1)
	if s.failN > 0 && n <= s.failN {
		return errors.New("notify down")
	}
	return nil
}

func TestHandleAlertMessageDoesNotAckWhenNotifyFailsBeforeDeadline(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, redisx.StreamLogs, redisx.GroupAlerter, "0").Err(); err != nil {
		t.Fatal(err)
	}

	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelError,
		Service:   "api",
		Message:   "panic: boom",
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
		Group:    redisx.GroupAlerter,
		Consumer: "alerter-test",
		Streams:  []string{redisx.StreamLogs, ">"},
		Count:    1,
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	msg := streams[0].Messages[0]
	if msg.ID != msgID {
		t.Fatalf("msg id = %q", msg.ID)
	}

	rules := []alertengine.Rule{{
		ID: "pat", Name: "Pattern", Enabled: true, Pattern: "panic",
		Threshold: 1, Cooldown: time.Minute, Channels: []string{"slack"},
	}}
	engine := alertengine.NewEngine(rules)
	notifier := &stubNotifier{failN: 100}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err = handleAlertMessage(runCtx, rdb, engine, notifier, msg, notifyOptions{maxAttempts: 5}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v want context canceled", err)
	}

	pending, err := rdb.XPending(ctx, redisx.StreamLogs, redisx.GroupAlerter).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pending.Count != 1 {
		t.Fatalf("pending = %d want 1 without ack on failure", pending.Count)
	}
}

func TestHandleAlertMessageDeadLettersAfterMaxAttempts(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, redisx.StreamLogs, redisx.GroupAlerter, "0").Err(); err != nil {
		t.Fatal(err)
	}

	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelError,
		Service:   "api",
		Message:   "panic: dead",
	}
	raw, _ := entry.ToJSON()
	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: redisx.StreamLogs,
		Values: map[string]interface{}{redisx.FieldPayload: raw},
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    redisx.GroupAlerter,
		Consumer: "alerter-dlq-test",
		Streams:  []string{redisx.StreamLogs, ">"},
		Count:    1,
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	msg := streams[0].Messages[0]

	rules := []alertengine.Rule{{
		ID: "pat", Name: "Pattern", Enabled: true, Pattern: "panic",
		Threshold: 1, Cooldown: time.Minute, Channels: []string{"slack"},
	}}
	engine := alertengine.NewEngine(rules)
	notifier := &stubNotifier{failN: 10}

	if err := handleAlertMessage(ctx, rdb, engine, notifier, msg, notifyOptions{maxAttempts: 2}, nil); err != nil {
		t.Fatal(err)
	}

	pending, err := rdb.XPending(ctx, redisx.StreamLogs, redisx.GroupAlerter).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pending.Count != 0 {
		t.Fatalf("pending = %d want 0 after dead letter ack", pending.Count)
	}

	dlqLen, err := rdb.XLen(ctx, redisx.StreamAlerterDLQ).Result()
	if err != nil {
		t.Fatal(err)
	}
	if dlqLen != 1 {
		t.Fatalf("dlq length = %d want 1", dlqLen)
	}
}

func TestHandleAlertMessageAcksAfterSuccessfulNotify(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, redisx.StreamLogs, redisx.GroupAlerter, "0").Err(); err != nil {
		t.Fatal(err)
	}

	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelError,
		Service:   "api",
		Message:   "panic: ok",
	}
	raw, _ := entry.ToJSON()
	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: redisx.StreamLogs,
		Values: map[string]interface{}{redisx.FieldPayload: raw},
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	streams, _ := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: redisx.GroupAlerter, Consumer: "ok-test",
		Streams: []string{redisx.StreamLogs, ">"}, Count: 1,
	}).Result()
	msg := streams[0].Messages[0]

	rules := []alertengine.Rule{{
		ID: "pat", Name: "Pattern", Enabled: true, Pattern: "panic",
		Threshold: 1, Cooldown: time.Minute, Channels: []string{"slack"},
	}}
	engine := alertengine.NewEngine(rules)
	if err := handleAlertMessage(ctx, rdb, engine, &stubNotifier{}, msg, notifyOptions{maxAttempts: 3}, nil); err != nil {
		t.Fatal(err)
	}
	pending, _ := rdb.XPending(ctx, redisx.StreamLogs, redisx.GroupAlerter).Result()
	if pending.Count != 0 {
		t.Fatalf("pending = %d want 0", pending.Count)
	}
}
