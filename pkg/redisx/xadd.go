package redisx

import "github.com/redis/go-redis/v9"

// LogStreamAddArgs builds XADD arguments with optional approximate MAXLEN trimming.
func LogStreamAddArgs(payload string, maxLen int64) *redis.XAddArgs {
	args := &redis.XAddArgs{
		Stream: StreamLogs,
		Values: map[string]interface{}{FieldPayload: payload},
	}
	if maxLen > 0 {
		args.MaxLen = maxLen
		args.Approx = true
	}
	return args
}
