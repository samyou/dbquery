package dbquery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type namedProfile struct {
	Name    string  `json:"name"`
	Profile Profile `json:"profile"`
}

type showPayload struct {
	Target       string         `json:"target"`
	SettingsFile string         `json:"settings_file,omitempty"`
	ProfilesFile string         `json:"profiles_file,omitempty"`
	Settings     *Settings      `json:"settings,omitempty"`
	Profiles     []namedProfile `json:"profiles"`
}

func runShow(cfg Config) error {
	payload := showPayload{Target: cfg.ShowTarget}

	if cfg.ShowTarget == "all" || cfg.ShowTarget == "settings" {
		settings, err := loadSettings(cfg.SettingsFile)
		if err != nil {
			return err
		}
		settings.APIKey = maskSecret(settings.APIKey)
		payload.SettingsFile = cfg.SettingsFile
		payload.Settings = &settings
	}

	if cfg.ShowTarget == "all" || cfg.ShowTarget == "profiles" {
		profiles, err := loadProfiles(cfg.ProfilesFile)
		if err != nil {
			return err
		}
		payload.ProfilesFile = cfg.ProfilesFile
		payload.Profiles = toNamedProfiles(profiles)
		if payload.Profiles == nil {
			payload.Profiles = make([]namedProfile, 0)
		}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal show output: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func maskSecret(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func toNamedProfiles(profiles map[string]Profile) []namedProfile {
	if len(profiles) == 0 {
		return nil
	}

	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]namedProfile, 0, len(names))
	for _, name := range names {
		out = append(out, namedProfile{Name: name, Profile: profiles[name]})
	}
	return out
}
