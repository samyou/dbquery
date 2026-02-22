package dbquery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Profile struct {
	DBType          string   `json:"db_type,omitempty"`
	DBURL           string   `json:"db_url,omitempty"`
	Output          string   `json:"output,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	Tables          []string `json:"tables,omitempty"`
	SchemaFile      string   `json:"schema_file,omitempty"`
	SchemaMaxTables int      `json:"schema_max_tables,omitempty"`

	Model       string  `json:"model,omitempty"`
	LLMBaseURL  string  `json:"llm_base_url,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Timeout     string  `json:"timeout,omitempty"`

	AllowWrite  bool `json:"allow_write,omitempty"`
	NoAutoLimit bool `json:"no_auto_limit,omitempty"`
}

func loadProfile(path, name string) (Profile, error) {
	profiles, err := loadProfiles(path)
	if err != nil {
		return Profile{}, err
	}

	key := strings.TrimSpace(name)
	if key == "" {
		return Profile{}, errors.New("profile name cannot be empty")
	}

	p, ok := profiles[key]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found in %s", key, path)
	}

	return p, nil
}

func loadProfiles(path string) (map[string]Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]Profile{}, nil
		}
		return nil, fmt.Errorf("read profiles file: %w", err)
	}

	profiles := make(map[string]Profile)
	if len(strings.TrimSpace(string(raw))) == 0 {
		return profiles, nil
	}

	if err := json.Unmarshal(raw, &profiles); err != nil {
		return nil, fmt.Errorf("parse profiles file: %w", err)
	}

	return profiles, nil
}

func saveProfile(path, name string, cfg Config) error {
	key := strings.TrimSpace(name)
	if key == "" {
		return errors.New("--save-profile cannot be empty")
	}

	profiles, err := loadProfiles(path)
	if err != nil {
		return err
	}

	profiles[key] = profileFromConfig(cfg)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}

	payload, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profiles: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write profiles file: %w", err)
	}

	return nil
}

func profileFromConfig(cfg Config) Profile {
	return Profile{
		DBType:          cfg.DBType,
		DBURL:           cfg.DBURL,
		Output:          cfg.Output,
		Limit:           cfg.Limit,
		Tables:          append([]string(nil), cfg.Tables...),
		SchemaFile:      cfg.SchemaFile,
		SchemaMaxTables: cfg.SchemaMaxTables,
		Model:           cfg.Model,
		LLMBaseURL:      cfg.LLMBaseURL,
		Temperature:     cfg.Temperature,
		MaxTokens:       cfg.MaxTokens,
		Timeout:         cfg.Timeout.String(),
		AllowWrite:      cfg.AllowWrite,
		NoAutoLimit:     cfg.NoAutoLimit,
	}
}

func applyProfileDefaults(cfg *Config, p Profile) {
	if strings.TrimSpace(p.DBType) != "" {
		cfg.DBType = strings.TrimSpace(p.DBType)
	}
	if strings.TrimSpace(p.DBURL) != "" {
		cfg.DBURL = strings.TrimSpace(p.DBURL)
	}
	if strings.TrimSpace(p.Output) != "" {
		cfg.Output = strings.TrimSpace(p.Output)
	}
	if p.Limit > 0 {
		cfg.Limit = p.Limit
	}
	if len(p.Tables) > 0 {
		cfg.Tables = append([]string(nil), p.Tables...)
	}
	if strings.TrimSpace(p.SchemaFile) != "" {
		cfg.SchemaFile = strings.TrimSpace(p.SchemaFile)
	}
	if p.SchemaMaxTables > 0 {
		cfg.SchemaMaxTables = p.SchemaMaxTables
	}

	if strings.TrimSpace(p.Model) != "" {
		cfg.Model = strings.TrimSpace(p.Model)
	}
	if strings.TrimSpace(p.LLMBaseURL) != "" {
		cfg.LLMBaseURL = strings.TrimSpace(p.LLMBaseURL)
	}
	if p.MaxTokens > 0 {
		cfg.MaxTokens = p.MaxTokens
	}
	if strings.TrimSpace(p.Timeout) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(p.Timeout))
		if err == nil && d > 0 {
			cfg.Timeout = d
		}
	}
	cfg.Temperature = p.Temperature

	cfg.AllowWrite = p.AllowWrite
	cfg.NoAutoLimit = p.NoAutoLimit
}
