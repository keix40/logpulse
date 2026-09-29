package logevent

import (
	"testing"
	"time"
)

func TestValidateBatch(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	valid := []Entry{{
		Timestamp: ts,
		Level:     "ERROR",
		Service:   "api",
		Message:   "boom",
	}}
	got, err := ValidateBatch(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Level != LevelError {
		t.Fatalf("expected normalized level error, got %s", got[0].Level)
	}
}

func TestValidateBatchRejectsEmpty(t *testing.T) {
	_, err := ValidateBatch(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEntryValidateRequiresService(t *testing.T) {
	e := Entry{Level: "info", Message: "hi"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
