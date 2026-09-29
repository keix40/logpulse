package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/logpulse/logpulse/pkg/ingestapi"
	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

func TestStreamPublisherTrimsWithApproxMaxLen(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	pub := &ingestapi.StreamPublisher{Client: rdb, StreamMaxLen: 2}
	ctx := context.Background()
	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelInfo,
		Service:   "ingest",
		Message:   "line",
	}

	for i := 0; i < 5; i++ {
		if err := pub.Publish(ctx, []logevent.Entry{entry}); err != nil {
			t.Fatal(err)
		}
	}

	n, err := rdb.XLen(ctx, redisx.StreamLogs).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n > 2 {
		t.Fatalf("stream length = %d want <= 2 with MAXLEN ~2", n)
	}
}
