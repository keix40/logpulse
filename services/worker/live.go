package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/logpulse/logpulse/pkg/logevent"
)

type LiveHub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewLiveHub() *LiveHub {
	return &LiveHub{clients: make(map[chan []byte]struct{})}
}

func (h *LiveHub) Run(_ context.Context, _ interface{}) {}

func (h *LiveHub) Broadcast(entry logevent.Entry) {
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- b:
		default:
		}
	}
}

func (h *LiveHub) SSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := make(chan []byte, 32)
		h.mu.Lock()
		h.clients[ch] = struct{}{}
		h.mu.Unlock()
		defer func() {
			h.mu.Lock()
			delete(h.clients, ch)
			h.mu.Unlock()
			close(ch)
		}()

		fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", msg)
				flusher.Flush()
			}
		}
	}
}
