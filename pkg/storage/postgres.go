package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/logpulse/logpulse/pkg/logevent"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgres(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &PostgresStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS pg_trgm`,
		`CREATE TABLE IF NOT EXISTS logs (
  id BIGSERIAL PRIMARY KEY,
  timestamp TIMESTAMPTZ NOT NULL,
  level TEXT NOT NULL,
  service TEXT NOT NULL,
  message TEXT NOT NULL,
  attributes JSONB NOT NULL DEFAULT '{}'::jsonb
)`,
		`CREATE INDEX IF NOT EXISTS logs_timestamp_idx ON logs (timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS logs_service_idx ON logs (service)`,
		`CREATE INDEX IF NOT EXISTS logs_level_idx ON logs (level)`,
		`CREATE INDEX IF NOT EXISTS logs_message_trgm_idx ON logs USING gin (message gin_trgm_ops)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) InsertBatch(ctx context.Context, entries []logevent.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO logs (timestamp, level, service, message, attributes)
VALUES ($1, $2, $3, $4, $5::jsonb)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		attrs := "{}"
		if len(e.Attributes) > 0 {
			b, err := json.Marshal(e.Attributes)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			attrs = string(b)
		}
		ts := e.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx, ts, strings.ToLower(e.Level), e.Service, e.Message, attrs); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) SearchHandler(logger *slog.Logger) http.HandlerFunc {
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
		argN := 1
		if q != "" {
			where = append(where, fmt.Sprintf("message ILIKE $%d", argN))
			args = append(args, "%"+q+"%")
			argN++
		}
		if level != "" {
			where = append(where, fmt.Sprintf("level = $%d", argN))
			args = append(args, strings.ToLower(level))
			argN++
		}
		if service != "" {
			where = append(where, fmt.Sprintf("service = $%d", argN))
			args = append(args, service)
			argN++
		}

		sqlQuery := fmt.Sprintf(`
SELECT timestamp, level, service, message, attributes::text
FROM logs
WHERE %s
ORDER BY timestamp DESC
LIMIT %d`, strings.Join(where, " AND "), limit)

		rows, err := s.db.QueryContext(r.Context(), sqlQuery, args...)
		if err != nil {
			if logger != nil {
				logger.Error("search query failed", "err", err)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
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
				if logger != nil {
					logger.Error("search scan failed", "err", err)
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
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

func (s *PostgresStore) RunRetentionLoop(ctx context.Context, retentionHours int, logger *slog.Logger) {
	if retentionHours <= 0 {
		retentionHours = 168
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	run := func() {
		cutoff := time.Now().UTC().Add(-time.Duration(retentionHours) * time.Hour)
		res, err := s.db.ExecContext(ctx, `DELETE FROM logs WHERE timestamp < $1`, cutoff)
		if err != nil {
			if logger != nil {
				logger.Error("retention delete failed", "err", err)
			}
			return
		}
		if n, _ := res.RowsAffected(); n > 0 && logger != nil {
			logger.Info("retention deleted rows", "count", n, "before", cutoff)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
