package ingestapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/logpulse/logpulse/pkg/logevent"
)

// Limits caps ingest request size.
type Limits struct {
	MaxBodyBytes int64
}

// Publisher enqueues validated log batches.
type Publisher interface {
	Publish(ctx context.Context, entries []logevent.Entry) error
}

func HandleBatch(p Publisher, limits Limits, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limits.MaxBodyBytes)
		var req logevent.BatchRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		logs, err := logevent.ValidateBatch(req.Logs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := p.Publish(r.Context(), logs); err != nil {
			if logger != nil {
				logger.Error("publish failed", "err", err)
			}
			http.Error(w, "failed to enqueue", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}
}
