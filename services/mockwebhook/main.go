package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

type server struct {
	mu       sync.Mutex
	requests []map[string]interface{}
}

func (s *server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	entry := map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
		"body":   string(body),
	}
	s.mu.Lock()
	s.requests = append(s.requests, entry)
	s.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleList(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := append([]map[string]interface{}(nil), s.requests...)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"requests": out})
}

func (s *server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	s.requests = nil
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	s := &server{}
	http.HandleFunc("/webhook", s.handleWebhook)
	http.HandleFunc("/v1/requests", s.handleList)
	http.HandleFunc("/v1/reset", s.handleReset)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	_ = http.ListenAndServe(":9090", nil)
}
