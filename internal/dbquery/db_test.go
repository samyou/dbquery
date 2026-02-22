package dbquery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLitePathFromDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		path string
		ok   bool
	}{
		{name: "memory shorthand", dsn: ":memory:", ok: false},
		{name: "memory file uri", dsn: "file::memory:?cache=shared", ok: false},
		{name: "plain path", dsn: "./example/sample.db", path: "./example/sample.db", ok: true},
		{name: "path with query", dsn: "./example/sample.db?cache=shared", path: "./example/sample.db", ok: true},
		{name: "postgres style file uri", dsn: "file:./example/sample.db", path: "./example/sample.db", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotOK := sqlitePathFromDSN(tt.dsn)
			if gotOK != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, gotOK)
			}
			if gotPath != tt.path {
				t.Fatalf("expected path=%q, got %q", tt.path, gotPath)
			}
		})
	}
}

func TestValidateSQLiteLocation(t *testing.T) {
	if err := validateSQLiteLocation(":memory:"); err != nil {
		t.Fatalf("memory dsn should be valid: %v", err)
	}

	missingPath := filepath.Join(t.TempDir(), "missing", "app.db")
	err := validateSQLiteLocation(missingPath)
	if err == nil {
		t.Fatal("expected error for missing sqlite path")
	}

	tmp := t.TempDir()
	dirPath := filepath.Join(tmp, "subdir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("create directory test fixture: %v", err)
	}
	err = validateSQLiteLocation(dirPath)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
	if !strings.Contains(err.Error(), "points to a directory") {
		t.Fatalf("expected directory error, got: %v", err)
	}
}
