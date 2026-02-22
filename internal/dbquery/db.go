package dbquery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DBTX interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func openDatabase(ctx context.Context, cfg Config) (*sql.DB, error) {
	var (
		driverName string
		dsn        string
	)

	switch cfg.DBType {
	case "sqlite":
		driverName = "sqlite"
		dsn = strings.TrimSpace(cfg.DBURL)
		if err := validateSQLiteLocation(dsn); err != nil {
			return nil, err
		}
	case "postgres":
		driverName = "pgx"
		dsn = strings.TrimSpace(cfg.DBURL)
	case "mysql":
		driverName = "mysql"
		dsn = strings.TrimSpace(cfg.DBURL)
	default:
		return nil, fmt.Errorf("unsupported database type %q", cfg.DBType)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(4)

	if cfg.DBType == "sqlite" {
		db.SetMaxIdleConns(1)
		db.SetMaxOpenConns(1)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		if cfg.DBType == "sqlite" {
			if strings.Contains(strings.ToLower(err.Error()), "unable to open database file") {
				path, ok := sqlitePathFromDSN(dsn)
				if ok {
					return nil, fmt.Errorf("unable to open sqlite database file %q: %w", path, err)
				}
				return nil, fmt.Errorf("unable to open sqlite database: %w", err)
			}
		}
		return nil, err
	}

	return db, nil
}

func validateSQLiteLocation(dsn string) error {
	path, ok := sqlitePathFromDSN(dsn)
	if !ok {
		return nil
	}

	if strings.TrimSpace(path) == "" {
		return nil
	}

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("sqlite path points to a directory: %s", path)
		}
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		parent := filepath.Dir(path)
		if parent != "." && parent != "" {
			if _, perr := os.Stat(parent); errors.Is(perr, os.ErrNotExist) {
				return fmt.Errorf("sqlite database directory does not exist: %s", parent)
			}
		}
		return fmt.Errorf("sqlite database file does not exist: %s", path)
	}

	return fmt.Errorf("check sqlite database path %q: %w", path, err)
}

func sqlitePathFromDSN(dsn string) (string, bool) {
	raw := strings.TrimSpace(dsn)
	if raw == "" {
		return "", false
	}

	lower := strings.ToLower(raw)
	if lower == ":memory:" || strings.HasPrefix(lower, "file::memory:") {
		return "", false
	}

	if strings.HasPrefix(lower, "file:") {
		u, err := url.Parse(raw)
		if err == nil {
			if strings.EqualFold(strings.TrimSpace(u.Query().Get("mode")), "memory") {
				return "", false
			}

			path := strings.TrimSpace(u.Path)
			if path == "" {
				path = strings.TrimSpace(u.Opaque)
			}
			if path == "" {
				path = strings.TrimPrefix(raw, "file:")
			}
			if decoded, err := url.PathUnescape(path); err == nil {
				path = decoded
			}
			path = strings.TrimSpace(path)
			if path == "" {
				return "", false
			}
			if path == ":memory:" {
				return "", false
			}
			return path, true
		}
	}

	if idx := strings.Index(raw, "?"); idx > 0 {
		return strings.TrimSpace(raw[:idx]), true
	}

	return raw, true
}

func executeQuery(ctx context.Context, db DBTX, query string) ([]string, []map[string]any, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		scanArgs := make([]any, len(columns))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, nil, err
		}

		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = normalizeDBValue(values[i])
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return columns, result, nil
}

func normalizeDBValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		return t.Format(time.RFC3339Nano)
	case []byte:
		s := string(t)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
		return s
	default:
		return t
	}
}
