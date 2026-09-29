package storage

import (
	"fmt"
	"os"
	"strconv"
)

// OpenFromEnv selects clickhouse or postgres based on STORAGE_BACKEND.
func OpenFromEnv() (Store, error) {
	backend := os.Getenv("STORAGE_BACKEND")
	if backend == "" {
		backend = "clickhouse"
	}
	switch backend {
	case "clickhouse":
		dsn := envOr("CLICKHOUSE_DSN", "clickhouse://default:logpulse@clickhouse:9000/default")
		return NewClickHouse(dsn)
	case "postgres":
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			return nil, fmt.Errorf("DATABASE_URL is required when STORAGE_BACKEND=postgres")
		}
		return NewPostgres(dsn)
	default:
		return nil, fmt.Errorf("unknown STORAGE_BACKEND %q", backend)
	}
}

// RetentionHoursFromEnv returns LOG_RETENTION_HOURS or default 168.
func RetentionHoursFromEnv() int {
	raw := os.Getenv("LOG_RETENTION_HOURS")
	if raw == "" {
		return 168
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 168
	}
	return n
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
