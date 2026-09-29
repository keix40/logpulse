package main

import (
	"fmt"

	"github.com/logpulse/logpulse/pkg/alertengine"
)

func formatIncidentMessage(inc alertengine.Incident) string {
	return fmt.Sprintf("[LogPulse] %s — %s (count=%d)\n%s",
		inc.RuleName, inc.RuleID, inc.Count, inc.Sample)
}
