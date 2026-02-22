package dbquery

import (
	"strings"
	"testing"
)

func TestRenderOutputJSON(t *testing.T) {
	columns := []string{"id", "name"}
	rows := []map[string]any{{"id": 1, "name": "sam"}}

	out, err := renderOutput("json", columns, rows)
	if err != nil {
		t.Fatalf("renderOutput returned error: %v", err)
	}
	if !strings.Contains(out, `"id": 1`) {
		t.Fatalf("json output missing id: %s", out)
	}
	if !strings.Contains(out, `"name": "sam"`) {
		t.Fatalf("json output missing name: %s", out)
	}
}

func TestRenderOutputTable(t *testing.T) {
	columns := []string{"id", "name"}
	rows := []map[string]any{{"id": 1, "name": "sam"}}

	out, err := renderOutput("table", columns, rows)
	if err != nil {
		t.Fatalf("renderOutput returned error: %v", err)
	}

	expected := []string{"| id", "| name", "sam"}
	for _, token := range expected {
		if !strings.Contains(out, token) {
			t.Fatalf("table output missing %q: %s", token, out)
		}
	}
}
