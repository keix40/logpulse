package main

import (
	"strconv"

	"github.com/logpulse/logpulse/pkg/ingestapi"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

func limitsFromConfig(cfg config) ingestapi.Limits {
	if cfg.MaxBodyBytes > 0 {
		return ingestapi.Limits{MaxBodyBytes: cfg.MaxBodyBytes}
	}
	return ingestapi.Limits{MaxBodyBytes: defaultMaxBodyBytes}
}

func parseMaxBodyBytes(raw string) int64 {
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
