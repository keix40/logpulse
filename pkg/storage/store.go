package storage

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/logpulse/logpulse/pkg/logevent"
)

// Store persists log entries and exposes search over HTTP.
type Store interface {
	InsertBatch(ctx context.Context, entries []logevent.Entry) error
	SearchHandler(logger *slog.Logger) http.HandlerFunc
	Close() error
}

// RetentionRunner optionally deletes old rows (Postgres free-tier sizing).
type RetentionRunner interface {
	RunRetentionLoop(ctx context.Context, retentionHours int, logger *slog.Logger)
}
