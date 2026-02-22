package dbquery

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HistoryEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Mode         string    `json:"mode"`
	DBType       string    `json:"db_type"`
	Profile      string    `json:"profile,omitempty"`
	NaturalQuery string    `json:"natural_query"`
	SQL          string    `json:"sql,omitempty"`
	Rows         int       `json:"rows"`
	DurationMs   int64     `json:"duration_ms"`
	Error        string    `json:"error,omitempty"`
}

func recordHistoryBestEffort(cfg Config, entry HistoryEntry) {
	if cfg.NoHistory || cfg.Mode == modeHistory {
		return
	}

	historyFile := strings.TrimSpace(cfg.HistoryFile)
	if historyFile == "" {
		historyFile = defaultHistoryFile()
	}

	if err := appendHistoryEntry(historyFile, entry); err != nil && cfg.Verbose {
		fmt.Fprintf(os.Stderr, "warning: failed to write history: %v\n", err)
	}
}

func appendHistoryEntry(path string, entry HistoryEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		return fmt.Errorf("encode history entry: %w", err)
	}
	return nil
}

func runHistory(cfg Config) error {
	entries, err := readHistoryEntries(cfg.HistoryFile)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No history entries found.")
		return nil
	}

	if cfg.HistoryLimit < len(entries) {
		entries = entries[len(entries)-cfg.HistoryLimit:]
	}

	if cfg.HistoryOutput == "json" {
		payload, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal history: %w", err)
		}
		fmt.Println(string(payload))
		return nil
	}

	columns := []string{"timestamp", "mode", "db", "rows", "ms", "query", "error"}
	if cfg.HistoryFull {
		columns = []string{"timestamp", "mode", "db", "rows", "ms", "query", "sql", "error"}
	}

	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		row := map[string]any{
			"timestamp": e.Timestamp.Format(time.RFC3339),
			"mode":      e.Mode,
			"db":        e.DBType,
			"rows":      e.Rows,
			"ms":        e.DurationMs,
			"query":     e.NaturalQuery,
			"error":     e.Error,
		}
		if cfg.HistoryFull {
			row["sql"] = e.SQL
		}
		rows = append(rows, row)
	}

	rendered, err := renderOutput("table", columns, rows)
	if err != nil {
		return err
	}
	fmt.Println(rendered)
	return nil
}

func readHistoryEntries(path string) ([]HistoryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	entries := make([]HistoryEntry, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse history entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read history file: %w", err)
	}

	return entries, nil
}
