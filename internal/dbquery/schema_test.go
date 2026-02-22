package dbquery

import (
	"context"
	"database/sql"
	"testing"
)

func TestIntrospectSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, created_at TEXT)`); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total REAL)`); err != nil {
		t.Fatalf("create orders table: %v", err)
	}

	tables, err := introspectSchema(ctx, db, "sqlite", nil, 10)
	if err != nil {
		t.Fatalf("introspectSchema returned error: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}

	scoped, err := introspectSchema(ctx, db, "sqlite", []string{"users"}, 10)
	if err != nil {
		t.Fatalf("introspectSchema with scope returned error: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("expected 1 scoped table, got %d", len(scoped))
	}
	if scoped[0].Name != "users" {
		t.Fatalf("expected users table, got %s", scoped[0].Name)
	}
	if len(scoped[0].Columns) == 0 {
		t.Fatal("expected users table to include columns")
	}
}
