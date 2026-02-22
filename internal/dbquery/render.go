package dbquery

import (
	"encoding/json"
	"fmt"
	"strings"
)

func renderOutput(format string, columns []string, rows []map[string]any) (string, error) {
	switch format {
	case "json":
		payload, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal json output: %w", err)
		}
		return string(payload), nil
	case "table":
		return renderTable(columns, rows), nil
	default:
		return "", fmt.Errorf("unsupported output format %q", format)
	}
}

func renderTable(columns []string, rows []map[string]any) string {
	if len(columns) == 0 {
		return "No rows returned."
	}

	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}

	stringRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		line := make([]string, len(columns))
		for i, col := range columns {
			v := formatCellValue(row[col])
			line[i] = v
			if len(v) > widths[i] {
				widths[i] = len(v)
			}
		}
		stringRows = append(stringRows, line)
	}

	hline := buildHorizontalLine(widths)
	var b strings.Builder
	b.WriteString(hline)
	b.WriteByte('\n')
	b.WriteString(buildTableRow(columns, widths))
	b.WriteByte('\n')
	b.WriteString(hline)
	b.WriteByte('\n')

	for _, line := range stringRows {
		b.WriteString(buildTableRow(line, widths))
		b.WriteByte('\n')
	}

	b.WriteString(hline)
	if len(rows) == 0 {
		b.WriteString("\n(0 rows)")
	}

	return b.String()
}

func buildHorizontalLine(widths []int) string {
	var b strings.Builder
	b.WriteByte('+')
	for _, w := range widths {
		b.WriteString(strings.Repeat("-", w+2))
		b.WriteByte('+')
	}
	return b.String()
}

func buildTableRow(values []string, widths []int) string {
	var b strings.Builder
	b.WriteByte('|')
	for i, v := range values {
		b.WriteByte(' ')
		b.WriteString(v)
		padding := widths[i] - len(v)
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteByte(' ')
		b.WriteByte('|')
	}
	return b.String()
}

func formatCellValue(v any) string {
	if v == nil {
		return "NULL"
	}

	str := fmt.Sprintf("%v", v)
	str = strings.ReplaceAll(str, "\n", " ")
	str = strings.ReplaceAll(str, "\r", " ")
	if str == "" {
		return ""
	}
	return str
}
