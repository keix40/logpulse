package main

import (
	"strconv"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

type ingestLimits struct {
	MaxBodyBytes int64
}

func limitsFromConfig(cfg config) ingestLimits {
	if cfg.MaxBodyBytes > 0 {
		return ingestLimits{MaxBodyBytes: cfg.MaxBodyBytes}
	}
	return ingestLimits{MaxBodyBytes: defaultMaxBodyBytes}
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
