package alertengine

import (
	"fmt"
	"testing"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

func TestSlidingWindowThreshold(t *testing.T) {
	rules := []Rule{{
		ID: "err-burst", Name: "Errors", Enabled: true, Level: "error",
		Window: time.Minute, Threshold: 3, Cooldown: time.Minute,
	}}
	e := NewEngine(rules)
	now := time.Now()
	entry := logevent.Entry{Level: "error", Service: "api", Message: "fail"}

	for i := 0; i < 2; i++ {
		if inc := e.Process(entry, now.Add(time.Duration(i)*time.Second)); len(inc) != 0 {
			t.Fatalf("unexpected incident at %d", i)
		}
	}
	inc := e.Process(entry, now.Add(2*time.Second))
	if len(inc) != 1 {
		t.Fatalf("expected incident, got %d", len(inc))
	}
}

func TestCooldownDedup(t *testing.T) {
	rules := []Rule{{
		ID: "pat", Name: "Pattern", Enabled: true, Pattern: "panic",
		Window: time.Minute, Threshold: 1, Cooldown: 10 * time.Minute,
	}}
	e := NewEngine(rules)
	now := time.Now()
	entry := logevent.Entry{Level: "error", Service: "api", Message: "panic: oops"}

	inc1 := e.Process(entry, now)
	if len(inc1) != 1 {
		t.Fatal("expected first incident")
	}
	inc2 := e.Process(entry, now.Add(time.Second))
	if len(inc2) != 0 {
		t.Fatal("expected cooldown to suppress duplicate")
	}
}

func TestThresholdCooldownGroupsByServiceNotMessage(t *testing.T) {
	rules := []Rule{{
		ID: "err-burst", Name: "Errors", Enabled: true, Level: "error",
		Window: time.Minute, Threshold: 3, Cooldown: 10 * time.Minute,
		GroupKey: "service",
	}}
	e := NewEngine(rules)
	now := time.Now()
	for i := 0; i < 3; i++ {
		e.Process(logevent.Entry{
			Level: "error", Service: "api", Message: fmt.Sprintf("fail-%d", i),
		}, now.Add(time.Duration(i)*time.Second))
	}
	inc := e.Process(logevent.Entry{Level: "error", Service: "api", Message: "fail-d"}, now.Add(3*time.Second))
	if len(inc) != 0 {
		t.Fatal("expected cooldown to suppress alerts for same service group")
	}
}

func TestIncidentCountReflectsWindowSize(t *testing.T) {
	rules := []Rule{{
		ID: "w", Name: "W", Enabled: true, Level: "error",
		Window: time.Minute, Threshold: 3, Cooldown: time.Minute,
	}}
	e := NewEngine(rules)
	now := time.Now()
	entry := logevent.Entry{Level: "error", Service: "s", Message: "x"}
	for i := 0; i < 2; i++ {
		e.Process(entry, now.Add(time.Duration(i)*time.Second))
	}
	inc := e.Process(entry, now.Add(2*time.Second))
	if len(inc) != 1 {
		t.Fatalf("expected incident, got %d", len(inc))
	}
	if inc[0].Count != 3 {
		t.Fatalf("count = %d want 3", inc[0].Count)
	}
}

func TestWindowExpiresOldEvents(t *testing.T) {
	rules := []Rule{{
		ID: "w", Name: "W", Enabled: true, Level: "error",
		Window: time.Minute, Threshold: 2, Cooldown: time.Minute,
	}}
	e := NewEngine(rules)
	base := time.Now()
	entry := logevent.Entry{Level: "error", Service: "s", Message: "x"}

	e.Process(entry, base)
	e.Process(entry, base.Add(30*time.Second))
	inc := e.Process(entry, base.Add(2*time.Minute))
	if len(inc) != 0 {
		t.Fatal("old events should have expired from window")
	}
}
