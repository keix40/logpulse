package logevent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
)

var validLevels = map[string]struct{}{
	LevelDebug: {},
	LevelInfo:  {},
	LevelWarn:  {},
	LevelError: {},
	LevelFatal: {},
}

// Entry is the canonical log record exchanged across LogPulse services.
type Entry struct {
	Timestamp  time.Time         `json:"timestamp"`
	Level      string            `json:"level"`
	Service    string            `json:"service"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// BatchRequest is the JSON body for POST /v1/logs.
type BatchRequest struct {
	Logs []Entry `json:"logs"`
}

func NormalizeLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

func (e *Entry) Validate() error {
	if e.Service == "" {
		return fmt.Errorf("service is required")
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Errorf("message is required")
	}
	lvl := NormalizeLevel(e.Level)
	if _, ok := validLevels[lvl]; !ok {
		return fmt.Errorf("invalid level %q", e.Level)
	}
	e.Level = lvl
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return nil
}

func ValidateBatch(logs []Entry) ([]Entry, error) {
	if len(logs) == 0 {
		return nil, fmt.Errorf("logs array must not be empty")
	}
	if len(logs) > 1000 {
		return nil, fmt.Errorf("batch size exceeds maximum of 1000")
	}
	out := make([]Entry, 0, len(logs))
	for i := range logs {
		if err := logs[i].Validate(); err != nil {
			return nil, fmt.Errorf("logs[%d]: %w", i, err)
		}
		out = append(out, logs[i])
	}
	return out, nil
}

func (e Entry) ToJSON() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ParseEntryJSON(raw string) (Entry, error) {
	var e Entry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return Entry{}, err
	}
	if err := e.Validate(); err != nil {
		return Entry{}, err
	}
	return e, nil
}
