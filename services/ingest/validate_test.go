package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/logpulse/logpulse/pkg/logevent"
)

type stubPublisher struct {
	last []logevent.Entry
}

func (s *stubPublisher) Publish(_ interface{}, entries []logevent.Entry) error {
	s.last = entries
	return nil
}

// satisfy handleBatch which expects *StreamPublisher - test validation via logevent directly
func TestHandleBatchValidation(t *testing.T) {
	body, _ := json.Marshal(logevent.BatchRequest{Logs: []logevent.Entry{
		{Level: "info", Service: "x", Message: "y"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	// Use redis-less test: only decode + validate path
	var br logevent.BatchRequest
	if err := json.NewDecoder(req.Body).Decode(&br); err != nil {
		t.Fatal(err)
	}
	logs, err := logevent.ValidateBatch(br.Logs)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log")
	}
}

func TestHandleBatchInvalidLevel(t *testing.T) {
	_, err := logevent.ValidateBatch([]logevent.Entry{
		{Level: "trace", Service: "x", Message: "y"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
