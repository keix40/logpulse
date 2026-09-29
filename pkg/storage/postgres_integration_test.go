//go:build integration

package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

func TestPostgresStoreRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	store, err := NewPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	marker := "pg_integration_" + time.Now().Format("150405")
	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelInfo,
		Service:   "ci",
		Message:   marker,
	}
	if err := store.InsertBatch(context.Background(), []logevent.Entry{entry}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/logs/search?q="+marker+"&limit=5", nil)
	rec := httptest.NewRecorder()
	store.SearchHandler(nil)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), marker) {
		t.Fatalf("search body missing marker: %s", rec.Body.String())
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
