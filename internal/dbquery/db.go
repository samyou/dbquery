package dbquery

import (
	"context"
	"database/sql"
	"fmt"
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
		return nil, err
	}

	return db, nil
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
