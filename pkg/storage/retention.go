package storage

import (
	"context"
	"log/slog"
)

// StartRetentionLoop runs TTL deletes when store is Postgres.
func StartRetentionLoop(ctx context.Context, store Store, logger *slog.Logger) {
	if pg, ok := store.(*PostgresStore); ok {
		go pg.RunRetentionLoop(ctx, RetentionHoursFromEnv(), logger)
	}
}
