package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/logpulse/logpulse/pkg/alertengine"
)

func TestFormatIncidentMessage(t *testing.T) {
	msg := formatIncidentMessage(alertengine.Incident{
		RuleName: "Smoke",
		RuleID:   "e2e",
		Count:    3,
		Sample:   "panic: boom",
	})
	if !strings.Contains(msg, "Smoke") || !strings.Contains(msg, "panic: boom") {
		t.Fatalf("unexpected format: %q", msg)
	}
}

func TestNotifierSlackWebhook(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewNotifierWithClient(config{SlackWebhookURL: srv.URL}, slog.New(slog.NewTextHandler(os.Stderr, nil)), srv.Client())
	err := n.Notify(context.Background(), alertengine.Incident{
		RuleName: "Test",
		RuleID:   "t1",
		Count:    1,
		Sample:   "E2E_SMOKE_ALERT",
		Channels: []string{"slack"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text, _ := body["text"].(string)
	if !strings.Contains(text, "E2E_SMOKE_ALERT") {
		t.Fatalf("payload text = %v", body["text"])
	}
}
