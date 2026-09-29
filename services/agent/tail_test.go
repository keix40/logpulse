package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTailFileSendsBatches(t *testing.T) {
	var mu sync.Mutex
	var batches [][]map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(b, &payload)
		logs, _ := payload["logs"].([]interface{})
		mu.Lock()
		for _, raw := range logs {
			m, _ := raw.(map[string]interface{})
			batches = append(batches, []map[string]interface{}{m})
		}
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	ingest := &ingestClient{url: srv.URL, client: srv.Client()}
	go func() {
		_ = tailFile(ctx, path, "billing", 1, 100*time.Millisecond, ingest, nil)
	}()

	time.Sleep(400 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("info line one\nerror: something failed\nwarn: disk almost full\n")
	_ = f.Close()
	time.Sleep(1200 * time.Millisecond)
	cancel()

	mu.Lock()
	n := len(batches)
	mu.Unlock()
	if n < 2 {
		t.Fatalf("expected multiple batches, got %d", n)
	}
}

func TestDefaultLineParserInfersError(t *testing.T) {
	e := defaultLineParser("error: timeout", "app")
	if e.Level != "error" || e.Service != "app" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}
