package dbquery

import "testing"

func TestNormalizeDBTypeInput(t *testing.T) {
	tests := []struct {
		in      string
		out     string
		wantErr bool
	}{
		{in: "sqlite", out: "sqlite"},
		{in: "postgres", out: "postgres"},
		{in: "postgresql", out: "postgres"},
		{in: "pg", out: "postgres"},
		{in: "mysql", out: "mysql"},
		{in: "oracle", wantErr: true},
	}

	for _, tt := range tests {
		got, err := normalizeDBTypeInput(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for input %q", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tt.in, err)
		}
		if got != tt.out {
			t.Fatalf("expected %q, got %q", tt.out, got)
		}
	}
}

func TestApplySettingsDefaults(t *testing.T) {
	cfg := Config{}
	s := Settings{APIKey: "k1", DBType: "sqlite", DBURL: "./app.db"}
	applySettingsDefaults(&cfg, s)

	if cfg.APIKey != "k1" || cfg.DBType != "sqlite" || cfg.DBURL != "./app.db" {
		t.Fatalf("settings defaults were not applied correctly: %+v", cfg)
	}

	cfg = Config{APIKey: "inline", DBType: "postgres", DBURL: "postgres://x"}
	applySettingsDefaults(&cfg, s)

	if cfg.APIKey != "inline" || cfg.DBType != "postgres" || cfg.DBURL != "postgres://x" {
		t.Fatalf("existing config should not be overridden: %+v", cfg)
	}
}

func TestDetectDBTypeFromLocation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "postgres url", input: "postgresql://user:pass@localhost:5432/app", want: "postgres"},
		{name: "mysql dsn", input: "user:pass@tcp(localhost:3306)/app?parseTime=true", want: "mysql"},
		{name: "sqlite path", input: "./example/app.db", want: "sqlite"},
		{name: "sqlite file uri", input: "file:app.db", want: "sqlite"},
		{name: "unknown", input: "something-random", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectDBTypeFromLocation(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseSetConfigDBAutoDetect(t *testing.T) {
	cfg, err := parseSetConfig([]string{"db", "postgresql://user:pass@localhost:5432/app"})
	if err != nil {
		t.Fatalf("parseSetConfig returned error: %v", err)
	}
	if cfg.SetTarget != "db" || cfg.SetDBType != "postgres" {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}

	cfg, err = parseSetConfig([]string{"db", "sqlite", "./example/app.db"})
	if err != nil {
		t.Fatalf("parseSetConfig explicit type returned error: %v", err)
	}
	if cfg.SetDBType != "sqlite" || cfg.SetDBURL != "./example/app.db" {
		t.Fatalf("unexpected parsed explicit config: %+v", cfg)
	}
}
