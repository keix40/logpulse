package main

import (
	"testing"

	"github.com/logpulse/logpulse/pkg/ingestapi"
)

func TestParseSyslogLineRFC5424(t *testing.T) {
	line := "<134>1 2026-09-29T12:00:00.000Z host e2e-syslog - - - E2E_SYSLOG_MARKER"
	entry := ingestapi.ParseSyslogLine(line)
	if entry.Service != "e2e-syslog" {
		t.Fatalf("service=%q", entry.Service)
	}
	if entry.Message != "E2E_SYSLOG_MARKER" && !contains(entry.Message, "E2E_SYSLOG_MARKER") {
		t.Fatalf("message=%q", entry.Message)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
