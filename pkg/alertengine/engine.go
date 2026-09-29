package alertengine

import (
	"sort"
	"sync"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

const (
	maxStateEntries   = 4096
	stateRetention    = 24 * time.Hour
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

	e.pruneState(now)

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

		windowCount := 1
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
			windowCount = len(kept)
			if windowCount < rule.Threshold {
				continue
			}
		}

		fp := fingerprint(rule, entry)
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
			Count:       windowCount,
			Sample:      entry.Message,
			TriggeredAt: now,
			Channels:    rule.Channels,
		})
	}
	return incidents
}

func fingerprint(rule Rule, entry logevent.Entry) string {
	return rule.ID + "|" + groupKey(rule, entry)
}

func groupKey(rule Rule, entry logevent.Entry) string {
	switch rule.GroupKey {
	case "message":
		return entry.Message
	case "service", "":
		return entry.Service
	default:
		if entry.Attributes != nil {
			if v, ok := entry.Attributes[rule.GroupKey]; ok && v != "" {
				return v
			}
		}
		return entry.Service
	}
}

func (e *Engine) pruneState(now time.Time) {
	for k, until := range e.cooldowns {
		if now.After(until) {
			delete(e.cooldowns, k)
		}
	}
	for k, last := range e.dedup {
		if now.Sub(last) > stateRetention {
			delete(e.dedup, k)
		}
	}
	e.boundMap(e.cooldowns, maxStateEntries)
	e.boundMap(e.dedup, maxStateEntries)
}

func (e *Engine) boundMap(m map[string]time.Time, max int) {
	if len(m) <= max {
		return
	}
	type kv struct {
		key string
		at  time.Time
	}
	items := make([]kv, 0, len(m))
	for k, v := range m {
		items = append(items, kv{key: k, at: v})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].at.Before(items[j].at)
	})
	for _, item := range items[:len(m)-max] {
		delete(m, item.key)
	}
}
