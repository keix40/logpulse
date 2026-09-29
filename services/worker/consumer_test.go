package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

type mockStore struct {
 batches [][]logevent.Entry
 err     error
}

func (m *mockStore) InsertBatch(_ context.Context, entries []logevent.Entry) error {
 if m.err != nil {
  return m.err
 }
 cp := append([]logevent.Entry(nil), entries...)
 m.batches = append(m.batches, cp)
 return nil
}

func TestFlushBatchPersistsAndFansOut(t *testing.T) {
 store := &mockStore{}
 hub := NewLiveHub()
 ch := make(chan []byte, 4)
 hub.mu.Lock()
 hub.clients[ch] = struct{}{}
 hub.mu.Unlock()

 entries := []logevent.Entry{{
  Timestamp: time.Now().UTC(),
  Level:     logevent.LevelInfo,
  Service:   "e2e",
  Message:   "hello",
 }}

 rest := flushBatch(context.Background(), store, hub, entries, nil)
 if len(rest) != 0 {
  t.Fatalf("expected empty batch after flush")
 }
 if len(store.batches) != 1 || len(store.batches[0]) != 1 {
  t.Fatalf("store did not receive batch")
 }

 select {
 case raw := <-ch:
  var got logevent.Entry
  if err := json.Unmarshal(raw, &got); err != nil {
   t.Fatal(err)
  }
  if got.Message != "hello" {
   t.Fatalf("broadcast message = %q", got.Message)
  }
 default:
  t.Fatal("expected live broadcast")
 }
}

func TestFlushBatchSkipsFanOutOnStoreError(t *testing.T) {
 store := &mockStore{err: errors.New("clickhouse down")}
 hub := NewLiveHub()
 ch := make(chan []byte, 1)
 hub.mu.Lock()
 hub.clients[ch] = struct{}{}
 hub.mu.Unlock()

 flushBatch(context.Background(), store, hub, []logevent.Entry{{
  Level: logevent.LevelInfo, Service: "s", Message: "m",
 }}, nil)
 select {
 case <-ch:
  t.Fatal("should not broadcast when store fails")
 default:
 }
}
