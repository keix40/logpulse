package alertengine

import (
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	ID          string        `yaml:"id"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description,omitempty"`
	Enabled     bool          `yaml:"enabled"`
	Service     string        `yaml:"service,omitempty"`
	Level       string        `yaml:"level,omitempty"`
	Window      time.Duration `yaml:"window"`
	Threshold   int           `yaml:"threshold,omitempty"`
	Pattern     string        `yaml:"pattern,omitempty"`
	Cooldown    time.Duration `yaml:"cooldown"`
	Channels    []string      `yaml:"channels"`

	patternRe *regexp.Regexp `yaml:"-"`
}

type RulesFile struct {
	Rules []Rule `yaml:"rules"`
}

func LoadRules(path string) ([]Rule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rf RulesFile
	if err := yaml.Unmarshal(b, &rf); err != nil {
		return nil, err
	}
	out := make([]Rule, 0, len(rf.Rules))
	for _, r := range rf.Rules {
		if r.ID == "" || r.Name == "" {
			continue
		}
		if r.Window <= 0 {
			r.Window = time.Minute
		}
		if r.Cooldown <= 0 {
			r.Cooldown = 5 * time.Minute
		}
		if r.Pattern != "" {
			re, err := regexp.Compile(r.Pattern)
			if err != nil {
				return nil, err
			}
			r.patternRe = re
		}
		if r.Threshold <= 0 && r.Pattern == "" {
			r.Threshold = 1
		}
		out = append(out, r)
	}
	return out, nil
}

func (r Rule) MatchesMessage(msg string) bool {
	if r.patternRe == nil {
		return true
	}
	return r.patternRe.MatchString(msg)
}
