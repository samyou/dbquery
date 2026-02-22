package dbquery

import "testing"

func TestEnsureReadOnlySQL(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "select ok", query: "SELECT * FROM users", wantErr: false},
		{name: "cte ok", query: "WITH recent AS (SELECT * FROM users) SELECT * FROM recent", wantErr: false},
		{name: "update blocked", query: "UPDATE users SET active = false", wantErr: true},
		{name: "delete blocked", query: "DELETE FROM users", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureReadOnlySQL(tt.query)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for query %q", tt.query)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for query %q: %v", tt.query, err)
			}
		})
	}
}

func TestEnsureLimit(t *testing.T) {
	q := ensureLimit("SELECT * FROM users", 10)
	if q != "SELECT * FROM users LIMIT 10;" {
		t.Fatalf("unexpected limited query: %q", q)
	}

	q = ensureLimit("SELECT * FROM users LIMIT 5", 10)
	if q != "SELECT * FROM users LIMIT 5" {
		t.Fatalf("existing limit should be preserved, got %q", q)
	}

	q = ensureLimit("UPDATE users SET active = true", 10)
	if q != "UPDATE users SET active = true" {
		t.Fatalf("non-select query should not be modified, got %q", q)
	}
}
