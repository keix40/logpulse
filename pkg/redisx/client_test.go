package redisx

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestNewClientFromEnvRedisAddr(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	t.Setenv("REDIS_ADDR", srv.Addr())

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientFromEnvRedisURL(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_URL", "redis://"+srv.Addr())

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientFromEnvInvalidURL(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_URL", "not-a-redis-url")
	if _, err := NewClientFromEnv(); err == nil {
		t.Fatal("expected error")
	}
}
