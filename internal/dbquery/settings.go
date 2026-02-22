package dbquery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	APIKey string `json:"api_key,omitempty"`
	DBType string `json:"db_type,omitempty"`
	DBURL  string `json:"db_url,omitempty"`
}

func runSet(cfg Config) error {
	settings, err := loadSettings(cfg.SettingsFile)
	if err != nil {
		return err
	}

	switch cfg.SetTarget {
	case "llm-key":
		settings.APIKey = cfg.SetLLMKey
		if err := saveSettings(cfg.SettingsFile, settings); err != nil {
			return err
		}
		fmt.Printf("Saved default LLM key to %s\n", cfg.SettingsFile)
		return nil
	case "db":
		settings.DBType = cfg.SetDBType
		settings.DBURL = cfg.SetDBURL
		if err := saveSettings(cfg.SettingsFile, settings); err != nil {
			return err
		}
		fmt.Printf("Saved default DB settings (%s) to %s\n", cfg.SetDBType, cfg.SettingsFile)
		return nil
	default:
		return fmt.Errorf("unsupported set target %q", cfg.SetTarget)
	}
}

func loadSettings(path string) (Settings, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("read settings file: %w", err)
	}

	if strings.TrimSpace(string(raw)) == "" {
		return Settings{}, nil
	}

	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, fmt.Errorf("parse settings file: %w", err)
	}

	if strings.TrimSpace(s.DBType) != "" {
		normalized, err := normalizeDBTypeInput(s.DBType)
		if err != nil {
			return Settings{}, fmt.Errorf("invalid db_type in settings file: %w", err)
		}
		s.DBType = normalized
	}

	s.APIKey = strings.TrimSpace(s.APIKey)
	s.DBURL = strings.TrimSpace(s.DBURL)

	return s, nil
}

func saveSettings(path string, s Settings) error {
	if strings.TrimSpace(s.DBType) != "" {
		normalized, err := normalizeDBTypeInput(s.DBType)
		if err != nil {
			return err
		}
		s.DBType = normalized
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	payload, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}

	return nil
}

func applySettingsDefaults(cfg *Config, s Settings) {
	if strings.TrimSpace(cfg.APIKey) == "" && strings.TrimSpace(s.APIKey) != "" {
		cfg.APIKey = strings.TrimSpace(s.APIKey)
	}
	if strings.TrimSpace(cfg.DBType) == "" && strings.TrimSpace(s.DBType) != "" {
		cfg.DBType = strings.TrimSpace(s.DBType)
	}
	if strings.TrimSpace(cfg.DBURL) == "" && strings.TrimSpace(s.DBURL) != "" {
		cfg.DBURL = strings.TrimSpace(s.DBURL)
	}
}
