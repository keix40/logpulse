package livehub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

func TestLiveHubBroadcastToMultipleClients(t *testing.T) {
	hub := New()
	ch1 := make(chan []byte, 2)
	ch2 := make(chan []byte, 2)
	hub.SubscribeTestChannel(ch1)
	hub.SubscribeTestChannel(ch2)

	hub.Broadcast(logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelWarn,
		Service:   "api",
		Message:   "fan-out",
	})

	for _, ch := range []chan []byte{ch1, ch2} {
		select {
		case b := <-ch:
			var e logevent.Entry
			if err := json.Unmarshal(b, &e); err != nil {
				t.Fatal(err)
			}
			if e.Message != "fan-out" {
				t.Fatalf("unexpected message %q", e.Message)
			}
		default:
			t.Fatal("client missed broadcast")
		}
	}
}
