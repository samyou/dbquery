package dbquery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseResetConfig(t *testing.T) {
	cfg, err := parseResetConfig([]string{"config"})
	if err != nil {
		t.Fatalf("parseResetConfig returned error: %v", err)
	}
	if cfg.ResetTarget != "config" {
		t.Fatalf("expected target config, got %q", cfg.ResetTarget)
	}

	cfg, err = parseResetConfig([]string{"-y"})
	if err != nil {
		t.Fatalf("parseResetConfig -y returned error: %v", err)
	}
	if !cfg.Yes {
		t.Fatal("expected Yes=true with -y")
	}
	if cfg.ResetTarget != "all" {
		t.Fatalf("expected default target all, got %q", cfg.ResetTarget)
	}

	cfg, err = parseResetConfig([]string{"-y", "config", "--settings-file", "/tmp/s.json"})
	if err != nil {
		t.Fatalf("parseResetConfig mixed order returned error: %v", err)
	}
	if !cfg.Yes || cfg.ResetTarget != "config" {
		t.Fatalf("unexpected mixed-order parse result: %+v", cfg)
	}

	cfg, err = parseResetConfig([]string{"--dry-run", "all"})
	if err != nil {
		t.Fatalf("parseResetConfig dry-run returned error: %v", err)
	}
	if !cfg.DryRun || cfg.ResetTarget != "all" {
		t.Fatalf("unexpected dry-run parse result: %+v", cfg)
	}
}

func TestResetItemsForTarget(t *testing.T) {
	cfg := Config{
		SettingsFile: "/tmp/settings.json",
		ProfilesFile: "/tmp/profiles.json",
		HistoryFile:  "/tmp/history.jsonl",
	}

	cfg.ResetTarget = "config"
	items := resetItemsForTarget(cfg)
	if len(items) != 1 || items[0].label != "config" {
		t.Fatalf("unexpected config reset items: %+v", items)
	}

	cfg.ResetTarget = "profile"
	items = resetItemsForTarget(cfg)
	if len(items) != 1 || items[0].label != "profile" {
		t.Fatalf("unexpected profile reset items: %+v", items)
	}

	cfg.ResetTarget = "all"
	items = resetItemsForTarget(cfg)
	if len(items) != 3 {
		t.Fatalf("expected 3 items for all, got %+v", items)
	}
}

func TestRunResetDryRunDoesNotDelete(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	profilesPath := filepath.Join(dir, "profiles.json")
	historyPath := filepath.Join(dir, "history.jsonl")

	if err := os.WriteFile(settingsPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write settings fixture: %v", err)
	}
	if err := os.WriteFile(profilesPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write profiles fixture: %v", err)
	}
	if err := os.WriteFile(historyPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write history fixture: %v", err)
	}

	cfg := Config{
		Mode:         modeReset,
		ResetTarget:  "all",
		Yes:          true,
		DryRun:       true,
		SettingsFile: settingsPath,
		ProfilesFile: profilesPath,
		HistoryFile:  historyPath,
	}

	if err := runReset(cfg); err != nil {
		t.Fatalf("runReset dry-run returned error: %v", err)
	}

	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file should remain in dry-run: %v", err)
	}
	if _, err := os.Stat(profilesPath); err != nil {
		t.Fatalf("profiles file should remain in dry-run: %v", err)
	}
	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("history file should remain in dry-run: %v", err)
	}
}
