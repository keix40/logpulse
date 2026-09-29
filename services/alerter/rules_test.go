package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/logpulse/logpulse/pkg/alertengine"
)

func TestLoadRulesFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	content := `
rules:
  - id: ci-smoke
    name: CI smoke
    enabled: true
    pattern: "E2E_SMOKE_ALERT"
    cooldown: 1m
    channels: [slack]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := alertengine.LoadRules(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].ID != "ci-smoke" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}
