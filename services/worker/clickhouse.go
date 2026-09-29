package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/logpulse/logpulse/pkg/logevent"
)

type ClickHouseStore struct {
	conn clickhouse.Conn
}

func NewClickHouseStore(dsn string) (*ClickHouseStore, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	store := &ClickHouseStore{conn: conn}
	if err := store.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}

func (s *ClickHouseStore) ensureSchema(ctx context.Context) error {
	q := `
CREATE TABLE IF NOT EXISTS logs (
  timestamp DateTime64(3, 'UTC'),
  level LowCardinality(String),
  service LowCardinality(String),
  message String,
  attributes String
) ENGINE = MergeTree()
ORDER BY (timestamp, service, level)
`
	return s.conn.Exec(ctx, q)
}

func (s *ClickHouseStore) InsertBatch(ctx context.Context, entries []logevent.Entry) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO logs (timestamp, level, service, message, attributes)")
	if err != nil {
		return err
	}
	for _, e := range entries {
		attrs := "{}"
		if len(e.Attributes) > 0 {
			b, _ := json.Marshal(e.Attributes)
			attrs = string(b)
		}
		if err := batch.Append(e.Timestamp, e.Level, e.Service, e.Message, attrs); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *ClickHouseStore) SearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		level := r.URL.Query().Get("level")
		service := r.URL.Query().Get("service")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}

		where := []string{"1=1"}
		args := []interface{}{}
		if q != "" {
			where = append(where, "positionCaseInsensitive(message, ?) > 0")
			args = append(args, q)
		}
		if level != "" {
			where = append(where, "level = ?")
			args = append(args, strings.ToLower(level))
		}
		if service != "" {
			where = append(where, "service = ?")
			args = append(args, service)
		}

		sql := fmt.Sprintf(`
SELECT timestamp, level, service, message, attributes
FROM logs
WHERE %s
ORDER BY timestamp DESC
LIMIT %d`, strings.Join(where, " AND "), limit)

		rows, err := s.conn.Query(r.Context(), sql, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type row struct {
			Timestamp  time.Time `json:"timestamp"`
			Level      string    `json:"level"`
			Service    string    `json:"service"`
			Message    string    `json:"message"`
			Attributes string    `json:"attributes"`
		}
		var out []row
		for rows.Next() {
			var ts time.Time
			var lvl, svc, msg, attrs string
			if err := rows.Scan(&ts, &lvl, &svc, &msg, &attrs); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			out = append(out, row{Timestamp: ts, Level: lvl, Service: svc, Message: msg, Attributes: attrs})
		}
		if out == nil {
			out = []row{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"logs": out})
	}
}
