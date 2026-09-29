package main

import "os"

type config struct {
	HTTPAddr     string
	SyslogAddr   string
	RedisAddr    string
	MaxBodyBytes int64
	StreamMaxLen int64
}

func loadConfig() config {
	return config{
		HTTPAddr:     envOr("INGEST_HTTP_ADDR", "0.0.0.0:8080"),
		SyslogAddr:   envOr("INGEST_SYSLOG_ADDR", "0.0.0.0:5514"),
		RedisAddr:    envOr("REDIS_ADDR", "redis:6379"),
		MaxBodyBytes: parseMaxBodyBytes(envOr("INGEST_MAX_BODY_BYTES", "")),
		StreamMaxLen: parseMaxBodyBytes(envOr("REDIS_STREAM_MAXLEN", "100000")),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
