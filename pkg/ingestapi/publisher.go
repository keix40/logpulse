package ingestapi

import (
	"context"
	"log/slog"

	"github.com/logpulse/logpulse/pkg/logevent"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/redis/go-redis/v9"
)

type StreamPublisher struct {
	Client       *redis.Client
	Logger       *slog.Logger
	StreamMaxLen int64
}

func (p *StreamPublisher) Publish(ctx context.Context, entries []logevent.Entry) error {
	pipe := p.Client.Pipeline()
	for _, e := range entries {
		payload, err := e.ToJSON()
		if err != nil {
			return err
		}
		pipe.XAdd(ctx, redisx.LogStreamAddArgs(payload, p.StreamMaxLen))
	}
	_, err := pipe.Exec(ctx)
	return err
}
