package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/logpulse/logpulse/pkg/logevent"
)

type mockPublisher struct {
	last []logevent.Entry
	err  error
}

func (m *mockPublisher) Publish(_ context.Context, entries []logevent.Entry) error {
	if m.err != nil {
		return m.err
	}
	m.last = append([]logevent.Entry(nil), entries...)
	return nil
}

func TestHandleBatchAcceptsValidPayload(t *testing.T) {
	pub := &mockPublisher{}
	handler := handleBatch(pub, ingestLimits{MaxBodyBytes: 4096}, nil)

	body, _ := json.Marshal(logevent.BatchRequest{Logs: []logevent.Entry{
		{Level: "info", Service: "api", Message: "hello"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if len(pub.last) != 1 || pub.last[0].Message != "hello" {
		t.Fatalf("publisher got %+v", pub.last)
	}
}

func TestHandleBatchRejectsInvalidJSON(t *testing.T) {
	handler := handleBatch(&mockPublisher{}, ingestLimits{MaxBodyBytes: 4096}, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", strings.NewReader("{not-json"))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHandleBatchRejectsValidationError(t *testing.T) {
	handler := handleBatch(&mockPublisher{}, ingestLimits{MaxBodyBytes: 4096}, nil)
	body, _ := json.Marshal(logevent.BatchRequest{Logs: []logevent.Entry{
		{Level: "trace", Service: "api", Message: "x"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHandleBatchRejectsOversizedBody(t *testing.T) {
	handler := handleBatch(&mockPublisher{}, ingestLimits{MaxBodyBytes: 32}, nil)
	body := []byte(`{"logs":[{"level":"info","service":"s","message":"` + strings.Repeat("x", 64) + `"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d want 413", rec.Code)
	}
}

func TestHandleBatchRejectsLongMessageField(t *testing.T) {
	handler := handleBatch(&mockPublisher{}, ingestLimits{MaxBodyBytes: 1 << 20}, nil)
	body, _ := json.Marshal(logevent.BatchRequest{Logs: []logevent.Entry{
		{Level: "info", Service: "api", Message: strings.Repeat("m", logevent.MaxMessageLen+1)},
	}})
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHandleBatchRejectsOversizedBatch(t *testing.T) {
	handler := handleBatch(&mockPublisher{}, ingestLimits{MaxBodyBytes: 1 << 20}, nil)
	logs := make([]logevent.Entry, logevent.MaxBatchEntries+1)
	for i := range logs {
		logs[i] = logevent.Entry{Level: "info", Service: "s", Message: "m"}
	}
	body, _ := json.Marshal(logevent.BatchRequest{Logs: logs})
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}
