package dbquery

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

type tableDef struct {
	Name    string
	Columns []string
}

func buildSchemaContext(ctx context.Context, db *sql.DB, cfg Config) (string, error) {
	tables, err := introspectSchema(ctx, db, cfg.DBType, cfg.Tables, cfg.SchemaMaxTables)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("Discovered schema:\n")
	if len(tables) == 0 {
		b.WriteString("(no tables discovered)\n")
	} else {
		for _, t := range tables {
			b.WriteString("- ")
			b.WriteString(t.Name)
			b.WriteString(" (")
			b.WriteString(strings.Join(t.Columns, ", "))
			b.WriteString(")\n")
		}
	}

	if cfg.SchemaFile != "" {
		content, err := os.ReadFile(cfg.SchemaFile)
		if err != nil {
			return "", fmt.Errorf("read --schema-file: %w", err)
		}
		b.WriteString("\nExtra schema context from file:\n")
		b.WriteString(string(content))
		if !strings.HasSuffix(string(content), "\n") {
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}

func introspectSchema(ctx context.Context, db *sql.DB, dbType string, tableScope []string, maxTables int) ([]tableDef, error) {
	filter := makeTableFilter(tableScope)

	switch dbType {
	case "sqlite":
		return introspectSQLite(ctx, db, filter, maxTables)
	case "postgres":
		return introspectPostgres(ctx, db, filter, maxTables)
	case "mysql":
		return introspectMySQL(ctx, db, filter, maxTables)
	default:
		return nil, fmt.Errorf("unsupported db type %q", dbType)
	}
}

func introspectSQLite(ctx context.Context, db *sql.DB, filter map[string]struct{}, maxTables int) ([]tableDef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}

	tableNames := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !allowTableName(tableName, filter) {
			continue
		}
		tableNames = append(tableNames, tableName)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	out := make([]tableDef, 0)
	for _, tableName := range tableNames {
		escaped := strings.ReplaceAll(tableName, `"`, `""`)
		metaQuery := fmt.Sprintf(`SELECT * FROM "%s" LIMIT 0`, escaped)
		colRows, err := db.QueryContext(ctx, metaQuery)
		if err != nil {
			return nil, err
		}

		colNames, err := colRows.Columns()
		if err != nil {
			_ = colRows.Close()
			return nil, err
		}
		colTypes, err := colRows.ColumnTypes()
		if err != nil {
			_ = colRows.Close()
			return nil, err
		}
		if err := colRows.Close(); err != nil {
			return nil, err
		}

		columns := make([]string, 0, len(colNames))
		for i, name := range colNames {
			colType := ""
			if i < len(colTypes) {
				colType = colTypes[i].DatabaseTypeName()
			}
			colDesc := strings.TrimSpace(name + " " + colType)
			columns = append(columns, strings.TrimSpace(colDesc))
		}

		out = append(out, tableDef{Name: tableName, Columns: columns})
		if len(out) >= maxTables {
			break
		}
	}
	return out, nil
}

func introspectPostgres(ctx context.Context, db *sql.DB, filter map[string]struct{}, maxTables int) ([]tableDef, error) {
	tableRows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_type = 'BASE TABLE'
		  AND table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name`)
	if err != nil {
		return nil, err
	}
	defer tableRows.Close()

	out := make([]tableDef, 0)
	for tableRows.Next() {
		var schemaName, tableName string
		if err := tableRows.Scan(&schemaName, &tableName); err != nil {
			return nil, err
		}

		fullName := schemaName + "." + tableName
		if !allowTableName(fullName, filter) {
			continue
		}

		colRows, err := db.QueryContext(ctx, `
			SELECT column_name, data_type
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY ordinal_position`, schemaName, tableName)
		if err != nil {
			return nil, err
		}

		columns := make([]string, 0)
		for colRows.Next() {
			var colName, dataType string
			if err := colRows.Scan(&colName, &dataType); err != nil {
				_ = colRows.Close()
				return nil, err
			}
			columns = append(columns, strings.TrimSpace(colName+" "+dataType))
		}
		if err := colRows.Err(); err != nil {
			_ = colRows.Close()
			return nil, err
		}
		_ = colRows.Close()

		out = append(out, tableDef{Name: fullName, Columns: columns})
		if len(out) >= maxTables {
			break
		}
	}

	if err := tableRows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func introspectMySQL(ctx context.Context, db *sql.DB, filter map[string]struct{}, maxTables int) ([]tableDef, error) {
	tableRows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer tableRows.Close()

	out := make([]tableDef, 0)
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			return nil, err
		}
		if !allowTableName(tableName, filter) {
			continue
		}

		colRows, err := db.QueryContext(ctx, `
			SELECT column_name, data_type
			FROM information_schema.columns
			WHERE table_schema = DATABASE()
			  AND table_name = ?
			ORDER BY ordinal_position`, tableName)
		if err != nil {
			return nil, err
		}

		columns := make([]string, 0)
		for colRows.Next() {
			var colName, dataType string
			if err := colRows.Scan(&colName, &dataType); err != nil {
				_ = colRows.Close()
				return nil, err
			}
			columns = append(columns, strings.TrimSpace(colName+" "+dataType))
		}
		if err := colRows.Err(); err != nil {
			_ = colRows.Close()
			return nil, err
		}
		_ = colRows.Close()

		out = append(out, tableDef{Name: tableName, Columns: columns})
		if len(out) >= maxTables {
			break
		}
	}

	if err := tableRows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func makeTableFilter(tableScope []string) map[string]struct{} {
	if len(tableScope) == 0 {
		return nil
	}

	filter := make(map[string]struct{}, len(tableScope))
	for _, t := range tableScope {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		filter[t] = struct{}{}
	}

	if len(filter) == 0 {
		return nil
	}
	return filter
}

func allowTableName(name string, filter map[string]struct{}) bool {
	if len(filter) == 0 {
		return true
	}

	n := strings.ToLower(strings.TrimSpace(name))
	if _, ok := filter[n]; ok {
		return true
	}

	parts := strings.Split(n, ".")
	if len(parts) > 1 {
		if _, ok := filter[parts[len(parts)-1]]; ok {
			return true
		}
	}
	return false
}
