package main

import (
	"os"
	"strings"
)

type config struct {
	HTTPAddr      string
	RedisAddr     string
	ClickHouseDSN string
	CORSOrigins   []string
}

func loadConfig() config {
	origins := envOr("WORKER_CORS_ORIGINS", "http://localhost:3000")
	return config{
		HTTPAddr:      envOr("WORKER_HTTP_ADDR", "0.0.0.0:8081"),
		RedisAddr:     envOr("REDIS_ADDR", "redis:6379"),
		ClickHouseDSN: envOr("CLICKHOUSE_DSN", "clickhouse://default:@clickhouse:9000/logpulse"),
		CORSOrigins:   strings.Split(origins, ","),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
