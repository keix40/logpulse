package alertengine

import (
	"sync"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

type Incident struct {
	RuleID      string
	RuleName    string
	Fingerprint string
	Count       int
	Sample      string
	TriggeredAt time.Time
	Channels    []string
}

type Engine struct {
	mu        sync.Mutex
	rules     []Rule
	windows   map[string][]time.Time // ruleID -> event times
	cooldowns map[string]time.Time   // fingerprint -> until
	dedup     map[string]time.Time   // fingerprint -> last sent
}

func NewEngine(rules []Rule) *Engine {
	return &Engine{
		rules:     rules,
		windows:   make(map[string][]time.Time),
		cooldowns: make(map[string]time.Time),
		dedup:     make(map[string]time.Time),
	}
}

func (e *Engine) Process(entry logevent.Entry, now time.Time) []Incident {
	e.mu.Lock()
	defer e.mu.Unlock()

	var incidents []Incident
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Service != "" && entry.Service != rule.Service {
			continue
		}
		if rule.Level != "" && logevent.NormalizeLevel(entry.Level) != logevent.NormalizeLevel(rule.Level) {
			continue
		}
		if rule.Pattern != "" && !rule.MatchesMessage(entry.Message) {
			continue
		}

		if rule.Threshold > 0 {
			e.windows[rule.ID] = append(e.windows[rule.ID], now)
			cutoff := now.Add(-rule.Window)
			times := e.windows[rule.ID]
			kept := times[:0]
			for _, t := range times {
				if !t.Before(cutoff) {
					kept = append(kept, t)
				}
			}
			e.windows[rule.ID] = kept
			if len(kept) < rule.Threshold {
				continue
			}
		}

		fp := fingerprint(rule.ID, entry.Service, entry.Message)
		if until, ok := e.cooldowns[fp]; ok && now.Before(until) {
			continue
		}
		if last, ok := e.dedup[fp]; ok && now.Sub(last) < rule.Cooldown {
			continue
		}

		e.cooldowns[fp] = now.Add(rule.Cooldown)
		e.dedup[fp] = now
		if rule.Threshold > 0 {
			e.windows[rule.ID] = nil
		}

		incidents = append(incidents, Incident{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Fingerprint: fp,
			Count:       rule.Threshold,
			Sample:      entry.Message,
			TriggeredAt: now,
			Channels:    rule.Channels,
		})
	}
	return incidents
}

func fingerprint(ruleID, service, message string) string {
	return ruleID + "|" + service + "|" + message
}
