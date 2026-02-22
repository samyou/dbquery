package dbquery

import (
	"fmt"
	"regexp"
	"strings"
)

var forbiddenWritePattern = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|grant|revoke|merge|call|replace)\b`)
var hasLimitPattern = regexp.MustCompile(`(?i)\blimit\s+\d+`)

func ensureReadOnlySQL(query string) error {
	cleaned := stripLeadingComments(query)
	lower := strings.ToLower(strings.TrimSpace(cleaned))

	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with") || strings.HasPrefix(lower, "explain select")) {
		return fmt.Errorf("generated SQL is not read-only; use --allow-write to permit non-SELECT statements")
	}

	if forbiddenWritePattern.MatchString(lower) {
		return fmt.Errorf("generated SQL contains write/DDL keywords; use --allow-write if intentional")
	}

	return nil
}

func ensureLimit(query string, limit int) string {
	if limit <= 0 {
		return query
	}

	cleaned := stripLeadingComments(query)
	lower := strings.ToLower(strings.TrimSpace(cleaned))
	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return query
	}

	if hasLimitPattern.MatchString(lower) {
		return query
	}

	trimmed := strings.TrimSpace(query)
	trimmed = strings.TrimSuffix(trimmed, ";")
	return fmt.Sprintf("%s LIMIT %d;", trimmed, limit)
}

func stripLeadingComments(sqlText string) string {
	s := strings.TrimSpace(sqlText)
	for {
		s = strings.TrimSpace(s)

		if strings.HasPrefix(s, "--") {
			idx := strings.Index(s, "\n")
			if idx == -1 {
				return ""
			}
			s = s[idx+1:]
			continue
		}

		if strings.HasPrefix(s, "/*") {
			idx := strings.Index(s, "*/")
			if idx == -1 {
				return ""
			}
			s = s[idx+2:]
			continue
		}

		return s
	}
}
