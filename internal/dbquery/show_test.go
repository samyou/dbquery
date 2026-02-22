package dbquery

import "testing"

func TestParseShowConfig(t *testing.T) {
	cfg, err := parseShowConfig(nil)
	if err != nil {
		t.Fatalf("parseShowConfig returned error: %v", err)
	}
	if cfg.ShowTarget != "all" {
		t.Fatalf("expected default target all, got %q", cfg.ShowTarget)
	}

	cfg, err = parseShowConfig([]string{"settings"})
	if err != nil {
		t.Fatalf("parseShowConfig settings returned error: %v", err)
	}
	if cfg.ShowTarget != "settings" {
		t.Fatalf("expected settings target, got %q", cfg.ShowTarget)
	}

	cfg, err = parseShowConfig([]string{"--profiles-file", "/tmp/p.json", "profiles"})
	if err != nil {
		t.Fatalf("parseShowConfig mixed order returned error: %v", err)
	}
	if cfg.ShowTarget != "profiles" {
		t.Fatalf("expected profiles target, got %q", cfg.ShowTarget)
	}
}

func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "" {
		t.Fatalf("expected empty secret to stay empty, got %q", got)
	}
	if got := maskSecret("12345678"); got != "********" {
		t.Fatalf("expected full masking for short secret, got %q", got)
	}
	if got := maskSecret("sk_1234567890"); got != "sk_1*****7890" {
		t.Fatalf("unexpected masked output: %q", got)
	}
}

func TestToNamedProfilesSorted(t *testing.T) {
	in := map[string]Profile{
		"zeta": {DBType: "sqlite"},
		"alpha": {
			DBType: "postgres",
		},
	}

	out := toNamedProfiles(in)

	if len(out) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(out))
	}
	if out[0].Name != "alpha" || out[1].Name != "zeta" {
		t.Fatalf("profiles are not sorted: %+v", out)
	}
}
