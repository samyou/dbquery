package dbquery

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	modeQuery   = "query"
	modeChat    = "chat"
	modeHistory = "history"
	modeSet     = "set"
	modeReset   = "reset"
	modeShow    = "show"
)

type Config struct {
	Mode string

	DBType          string
	DBURL           string
	NLQuery         string
	Output          string
	OutputFile      string
	Limit           int
	Tables          []string
	SchemaFile      string
	SchemaMaxTables int

	Model      string
	APIKey     string
	LLMBaseURL string

	Temperature float64
	MaxTokens   int
	Timeout     time.Duration

	DryRun      bool
	ShowSQL     bool
	Verbose     bool
	AllowWrite  bool
	NoAutoLimit bool

	Profile      string
	SaveProfile  string
	ProfilesFile string
	SettingsFile string

	HistoryFile   string
	NoHistory     bool
	HistoryLimit  int
	HistoryOutput string
	HistoryFull   bool

	SetTarget string
	SetLLMKey string
	SetDBType string
	SetDBURL  string

	ResetTarget string
	Yes         bool

	ShowTarget string
}

func Run() error {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	switch cfg.Mode {
	case modeHistory:
		return runHistory(cfg)
	case modeSet:
		return runSet(cfg)
	case modeReset:
		return runReset(cfg)
	case modeShow:
		return runShow(cfg)
	case modeChat:
		return runChat(cfg)
	case modeQuery:
		return runSingleQuery(cfg)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

func runSingleQuery(cfg Config) error {
	if strings.TrimSpace(cfg.NLQuery) == "" {
		if cfg.SaveProfile != "" {
			fmt.Fprintln(os.Stderr, "Profile saved. No query provided, skipping execution.")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	schemaContext, err := buildSchemaContext(ctx, db, cfg)
	if err != nil {
		return fmt.Errorf("build schema context: %w", err)
	}

	_, err = processNaturalLanguageQuery(context.Background(), db, cfg, schemaContext, cfg.NLQuery)
	return err
}

func runChat(cfg Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	db, err := openDatabase(ctx, cfg)
	cancel()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), cfg.Timeout)
	schemaContext, err := buildSchemaContext(ctx, db, cfg)
	cancel()
	if err != nil {
		return fmt.Errorf("build schema context: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Entering interactive mode. Type :help for commands.")

	if strings.TrimSpace(cfg.NLQuery) != "" {
		if _, err := processNaturalLanguageQuery(context.Background(), db, cfg, schemaContext, cfg.NLQuery); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "dbquery> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == ":exit" || input == ":quit" {
			break
		}
		if input == ":help" {
			printChatHelp()
			continue
		}

		if _, err := processNaturalLanguageQuery(context.Background(), db, cfg, schemaContext, input); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func processNaturalLanguageQuery(parent context.Context, db DBTX, cfg Config, schemaContext, nlQuery string) (HistoryEntry, error) {
	entry := HistoryEntry{
		Timestamp:    time.Now().UTC(),
		Mode:         cfg.Mode,
		DBType:       cfg.DBType,
		Profile:      cfg.Profile,
		NaturalQuery: nlQuery,
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
	defer cancel()

	sqlQuery, err := generateSQL(ctx, cfg, schemaContext, nlQuery)
	if err != nil {
		entry.DurationMs = time.Since(start).Milliseconds()
		entry.Error = err.Error()
		recordHistoryBestEffort(cfg, entry)
		return entry, fmt.Errorf("generate SQL with LLM: %w", err)
	}

	sqlQuery = normalizeSQL(sqlQuery)
	if sqlQuery == "" {
		err := errors.New("LLM returned an empty SQL query")
		entry.DurationMs = time.Since(start).Milliseconds()
		entry.Error = err.Error()
		recordHistoryBestEffort(cfg, entry)
		return entry, err
	}

	if !cfg.AllowWrite {
		if err := ensureReadOnlySQL(sqlQuery); err != nil {
			entry.SQL = sqlQuery
			entry.DurationMs = time.Since(start).Milliseconds()
			entry.Error = err.Error()
			recordHistoryBestEffort(cfg, entry)
			return entry, err
		}
	}

	if !cfg.NoAutoLimit {
		sqlQuery = ensureLimit(sqlQuery, cfg.Limit)
	}

	entry.SQL = sqlQuery

	if cfg.ShowSQL || cfg.Verbose || cfg.DryRun {
		fmt.Fprintf(os.Stderr, "Generated SQL:\n%s\n", sqlQuery)
	}

	if cfg.DryRun {
		entry.DurationMs = time.Since(start).Milliseconds()
		recordHistoryBestEffort(cfg, entry)
		return entry, nil
	}

	columns, rows, err := executeQuery(ctx, db, sqlQuery)
	if err != nil {
		entry.DurationMs = time.Since(start).Milliseconds()
		entry.Error = err.Error()
		recordHistoryBestEffort(cfg, entry)
		return entry, fmt.Errorf("execute SQL query: %w", err)
	}

	rendered, err := renderOutput(cfg.Output, columns, rows)
	if err != nil {
		entry.DurationMs = time.Since(start).Milliseconds()
		entry.Error = err.Error()
		recordHistoryBestEffort(cfg, entry)
		return entry, err
	}

	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, []byte(rendered), 0o644); err != nil {
			entry.DurationMs = time.Since(start).Milliseconds()
			entry.Error = err.Error()
			recordHistoryBestEffort(cfg, entry)
			return entry, fmt.Errorf("write output file: %w", err)
		}
	}

	fmt.Println(rendered)

	entry.Rows = len(rows)
	entry.DurationMs = time.Since(start).Milliseconds()
	recordHistoryBestEffort(cfg, entry)
	return entry, nil
}

func parseConfig(args []string) (Config, error) {
	if len(args) == 0 {
		return parseQueryConfig(modeQuery, nil)
	}

	mode := modeQuery
	if args[0] == modeChat || args[0] == modeHistory || args[0] == modeSet || args[0] == modeReset || args[0] == modeShow {
		mode = args[0]
		args = args[1:]
	}

	if mode == modeHistory {
		return parseHistoryConfig(args)
	}
	if mode == modeSet {
		return parseSetConfig(args)
	}
	if mode == modeReset {
		return parseResetConfig(args)
	}
	if mode == modeShow {
		return parseShowConfig(args)
	}

	return parseQueryConfig(mode, args)
}

func parseQueryConfig(mode string, args []string) (Config, error) {
	var cfg Config
	cfg.Mode = mode
	cfg.Output = "table"
	cfg.Limit = 10
	cfg.SchemaMaxTables = 40
	cfg.LLMBaseURL = "https://api.openai.com/v1"
	cfg.Temperature = 0.0
	cfg.MaxTokens = 500
	cfg.Timeout = 30 * time.Second
	cfg.ProfilesFile = defaultProfilesFile()
	cfg.SettingsFile = defaultSettingsFile()
	cfg.HistoryFile = defaultHistoryFile()

	defaultModel := "gpt-4o-mini"
	if envModel := strings.TrimSpace(os.Getenv("LLM_MODEL")); envModel != "" {
		defaultModel = envModel
	}
	cfg.Model = defaultModel

	if settingsFile, ok := scanStringFlag(args, "settings-file"); ok && strings.TrimSpace(settingsFile) != "" {
		cfg.SettingsFile = strings.TrimSpace(settingsFile)
	}

	settings, err := loadSettings(cfg.SettingsFile)
	if err != nil {
		return cfg, err
	}
	applySettingsDefaults(&cfg, settings)

	if profileFile, ok := scanStringFlag(args, "profiles-file"); ok && strings.TrimSpace(profileFile) != "" {
		cfg.ProfilesFile = strings.TrimSpace(profileFile)
	}
	if profileName, ok := scanStringFlag(args, "profile"); ok && strings.TrimSpace(profileName) != "" {
		p, err := loadProfile(cfg.ProfilesFile, strings.TrimSpace(profileName))
		if err != nil {
			return cfg, err
		}
		applyProfileDefaults(&cfg, p)
		cfg.Profile = strings.TrimSpace(profileName)
	}

	fs := flag.NewFlagSet("dbquery", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&cfg.DBType, "db-type", cfg.DBType, "Database type: sqlite, postgres, mysql")
	fs.StringVar(&cfg.DBURL, "db-url", cfg.DBURL, "Database connection URL or sqlite file path")
	fs.StringVar(&cfg.NLQuery, "query", cfg.NLQuery, "Natural language request")
	fs.StringVar(&cfg.Output, "output", cfg.Output, "Output format: table or json")
	fs.StringVar(&cfg.OutputFile, "output-file", cfg.OutputFile, "Write rendered result to file")
	fs.IntVar(&cfg.Limit, "limit", cfg.Limit, "Default max rows to return")
	fs.StringVar(&cfg.SchemaFile, "schema-file", cfg.SchemaFile, "Optional schema/context file to improve SQL generation")
	fs.IntVar(&cfg.SchemaMaxTables, "schema-max-tables", cfg.SchemaMaxTables, "Maximum number of tables to include in schema context")

	tableScope := strings.Join(cfg.Tables, ",")
	fs.StringVar(&tableScope, "tables", tableScope, "Comma-separated table names to scope schema and SQL generation")

	fs.StringVar(&cfg.Model, "model", cfg.Model, "LLM model name")
	fs.StringVar(&cfg.APIKey, "api-key", cfg.APIKey, "LLM API key (or set default with `dbquery set llm-key`)")
	fs.StringVar(&cfg.LLMBaseURL, "llm-base-url", cfg.LLMBaseURL, "OpenAI-compatible base URL")
	fs.Float64Var(&cfg.Temperature, "temperature", cfg.Temperature, "LLM temperature")
	fs.IntVar(&cfg.MaxTokens, "max-tokens", cfg.MaxTokens, "LLM max completion tokens")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "Timeout per query (e.g. 45s, 2m)")

	fs.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Generate SQL only, do not execute query")
	fs.BoolVar(&cfg.ShowSQL, "show-sql", cfg.ShowSQL, "Print generated SQL to stderr")
	fs.BoolVar(&cfg.Verbose, "verbose", cfg.Verbose, "Print extra logs")
	fs.BoolVar(&cfg.AllowWrite, "allow-write", cfg.AllowWrite, "Allow non-read-only SQL statements")
	fs.BoolVar(&cfg.NoAutoLimit, "no-auto-limit", cfg.NoAutoLimit, "Do not auto-append LIMIT when missing")

	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "Load settings from a saved profile")
	fs.StringVar(&cfg.SaveProfile, "save-profile", "", "Save current settings to a profile name")
	fs.StringVar(&cfg.ProfilesFile, "profiles-file", cfg.ProfilesFile, "Path to profiles JSON file")
	fs.StringVar(&cfg.SettingsFile, "settings-file", cfg.SettingsFile, "Path to defaults settings JSON file")

	fs.StringVar(&cfg.HistoryFile, "history-file", cfg.HistoryFile, "Path to history JSONL file")
	fs.BoolVar(&cfg.NoHistory, "no-history", false, "Disable query history recording")

	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "dbquery: natural language SQL CLI\n\n")
		if mode == modeChat {
			fmt.Fprintf(out, "Usage:\n")
			fmt.Fprintf(out, "  dbquery chat --db-type <sqlite|postgres|mysql> --db-url <url-or-file> [options]\n\n")
		} else {
			fmt.Fprintf(out, "Usage:\n")
			fmt.Fprintf(out, "  dbquery --db-type <sqlite|postgres|mysql> --db-url <url-or-file> --query \"...\" [options]\n\n")
		}
		fmt.Fprintf(out, "Examples:\n")
		fmt.Fprintf(out, "  dbquery --db-type sqlite --db-url ./app.db --query \"get all users created in the last 10 days\"\n")
		fmt.Fprintf(out, "  dbquery chat --profile dev\n\n")
		fmt.Fprintf(out, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if strings.TrimSpace(cfg.DBType) == "" {
		return cfg, errors.New("--db-type is required")
	}
	if strings.TrimSpace(cfg.DBURL) == "" {
		return cfg, errors.New("--db-url is required")
	}
	if mode == modeQuery && strings.TrimSpace(cfg.NLQuery) == "" && strings.TrimSpace(cfg.SaveProfile) == "" {
		return cfg, errors.New("--query is required")
	}

	cfg.DBType = strings.ToLower(strings.TrimSpace(cfg.DBType))
	normalizedDBType, err := normalizeDBTypeInput(cfg.DBType)
	if err != nil {
		return cfg, err
	}
	cfg.DBType = normalizedDBType

	cfg.Output = strings.ToLower(strings.TrimSpace(cfg.Output))
	if cfg.Output != "table" && cfg.Output != "json" {
		return cfg, fmt.Errorf("unsupported --output %q (expected table|json)", cfg.Output)
	}

	if cfg.Limit <= 0 {
		return cfg, errors.New("--limit must be > 0")
	}
	if cfg.SchemaMaxTables <= 0 {
		return cfg, errors.New("--schema-max-tables must be > 0")
	}
	if cfg.Timeout <= 0 {
		return cfg, errors.New("--timeout must be > 0")
	}
	if cfg.MaxTokens <= 0 {
		return cfg, errors.New("--max-tokens must be > 0")
	}

	cfg.APIKey = strings.TrimSpace(cfg.APIKey)

	requiresLLM := mode == modeChat || strings.TrimSpace(cfg.NLQuery) != ""
	if requiresLLM && cfg.APIKey == "" {
		return cfg, errors.New("missing API key: use --api-key or set a default with `dbquery set llm-key`")
	}

	cfg.Tables = splitAndTrimCSV(tableScope)

	if cfg.SaveProfile != "" {
		if err := saveProfile(cfg.ProfilesFile, strings.TrimSpace(cfg.SaveProfile), cfg); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func parseHistoryConfig(args []string) (Config, error) {
	cfg := Config{
		Mode:          modeHistory,
		HistoryFile:   defaultHistoryFile(),
		HistoryLimit:  20,
		HistoryOutput: "json",
	}

	fs := flag.NewFlagSet("dbquery history", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&cfg.HistoryFile, "history-file", cfg.HistoryFile, "Path to history JSONL file")
	fs.IntVar(&cfg.HistoryLimit, "limit", cfg.HistoryLimit, "Number of history entries to show")
	fs.StringVar(&cfg.HistoryOutput, "output", cfg.HistoryOutput, "Output format: table or json")
	fs.BoolVar(&cfg.HistoryFull, "full", false, "Include generated SQL in output")

	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage:\n")
		fmt.Fprintf(out, "  dbquery history [options]\n\n")
		fmt.Fprintf(out, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.HistoryOutput = strings.ToLower(strings.TrimSpace(cfg.HistoryOutput))
	if cfg.HistoryOutput != "table" && cfg.HistoryOutput != "json" {
		return cfg, fmt.Errorf("unsupported --output %q (expected table|json)", cfg.HistoryOutput)
	}
	if cfg.HistoryLimit <= 0 {
		return cfg, errors.New("--limit must be > 0")
	}

	return cfg, nil
}

func parseSetConfig(args []string) (Config, error) {
	cfg := Config{
		Mode:         modeSet,
		SettingsFile: defaultSettingsFile(),
	}

	fs := flag.NewFlagSet("dbquery set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.SettingsFile, "settings-file", cfg.SettingsFile, "Path to defaults settings JSON file")

	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage:\n")
		fmt.Fprintf(out, "  dbquery set [--settings-file <path>] llm-key <api-key>\n")
		fmt.Fprintf(out, "  dbquery set [--settings-file <path>] db <db-url-or-path>\n")
		fmt.Fprintf(out, "  dbquery set [--settings-file <path>] db <sqlite|postgres|postgresql|mysql> <db-url-or-path>\n")
		fmt.Fprintf(out, "  dbquery set [--settings-file <path>] db <sqlite|postgres|postgresql|mysql> <name> <db-url-or-path>\n\n")
		fmt.Fprintf(out, "Examples:\n")
		fmt.Fprintf(out, "  dbquery set llm-key sk_12345\n")
		fmt.Fprintf(out, "  dbquery set db postgresql://user:pass@localhost:5432/app?sslmode=disable\n")
		fmt.Fprintf(out, "  dbquery set db sqlite ./example/app.db\n")
		fmt.Fprintf(out, "  dbquery set db postgresql postgres postgres://user:pass@localhost:5432/app?sslmode=disable\n\n")
		fmt.Fprintf(out, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	parts := fs.Args()
	if len(parts) < 2 {
		return cfg, errors.New("invalid set command; run `dbquery set -h` for usage")
	}

	switch strings.ToLower(strings.TrimSpace(parts[0])) {
	case "llm-key":
		if len(parts) != 2 {
			return cfg, errors.New("usage: dbquery set llm-key <api-key>")
		}
		cfg.SetTarget = "llm-key"
		cfg.SetLLMKey = strings.TrimSpace(parts[1])
		if cfg.SetLLMKey == "" {
			return cfg, errors.New("llm-key cannot be empty")
		}
	case "db":
		if len(parts) < 2 || len(parts) > 4 {
			return cfg, errors.New("usage: dbquery set db <db-url-or-path> OR dbquery set db <db-type> <db-url-or-path> (optional name token is allowed)")
		}

		cfg.SetTarget = "db"

		switch len(parts) {
		case 2:
			cfg.SetDBURL = strings.TrimSpace(parts[1])
			detected, err := detectDBTypeFromLocation(cfg.SetDBURL)
			if err != nil {
				return cfg, err
			}
			cfg.SetDBType = detected
		case 3:
			if normalized, err := normalizeDBTypeInput(parts[1]); err == nil {
				cfg.SetDBType = normalized
				cfg.SetDBURL = strings.TrimSpace(parts[2])
			} else {
				return cfg, fmt.Errorf("unable to detect db type from %q; provide explicit db type: dbquery set db <sqlite|postgres|mysql> <db-url-or-path>", parts[1])
			}
		case 4:
			dbType, err := normalizeDBTypeInput(parts[1])
			if err != nil {
				return cfg, err
			}
			cfg.SetDBType = dbType
			cfg.SetDBURL = strings.TrimSpace(parts[3])
		}

		if cfg.SetDBURL == "" {
			return cfg, errors.New("db url/path cannot be empty")
		}
	default:
		return cfg, fmt.Errorf("unsupported set target %q (expected llm-key or db)", parts[0])
	}

	return cfg, nil
}

func parseResetConfig(args []string) (Config, error) {
	cfg := Config{
		Mode:         modeReset,
		ResetTarget:  "all",
		SettingsFile: defaultSettingsFile(),
		ProfilesFile: defaultProfilesFile(),
		HistoryFile:  defaultHistoryFile(),
	}

	fs := flag.NewFlagSet("dbquery reset", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&cfg.Yes, "y", false, "Skip confirmation prompt")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Preview reset actions without deleting files")
	fs.StringVar(&cfg.SettingsFile, "settings-file", cfg.SettingsFile, "Path to defaults settings JSON file")
	fs.StringVar(&cfg.ProfilesFile, "profiles-file", cfg.ProfilesFile, "Path to profiles JSON file")
	fs.StringVar(&cfg.HistoryFile, "history-file", cfg.HistoryFile, "Path to history JSONL file")

	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage:\n")
		fmt.Fprintf(out, "  dbquery reset config [options]\n")
		fmt.Fprintf(out, "  dbquery reset profile [options]\n")
		fmt.Fprintf(out, "  dbquery reset all [options]\n")
		fmt.Fprintf(out, "  dbquery reset -y [all]\n\n")
		fmt.Fprintf(out, "Options:\n")
		fs.PrintDefaults()
	}

	parseArgs := args
	if len(args) > 0 && isResetTargetToken(args[0]) {
		cfg.ResetTarget = strings.ToLower(strings.TrimSpace(args[0]))
		parseArgs = args[1:]
	}

	if err := fs.Parse(parseArgs); err != nil {
		return cfg, err
	}

	rest := fs.Args()
	if len(rest) > 1 {
		if len(rest) >= 1 && isResetTargetToken(rest[0]) {
			cfg.ResetTarget = strings.ToLower(strings.TrimSpace(rest[0]))
			retryArgs := removeFirstArg(parseArgs, rest[0])
			fs = flag.NewFlagSet("dbquery reset", flag.ContinueOnError)
			fs.SetOutput(os.Stderr)
			fs.BoolVar(&cfg.Yes, "y", cfg.Yes, "Skip confirmation prompt")
			fs.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Preview reset actions without deleting files")
			fs.StringVar(&cfg.SettingsFile, "settings-file", cfg.SettingsFile, "Path to defaults settings JSON file")
			fs.StringVar(&cfg.ProfilesFile, "profiles-file", cfg.ProfilesFile, "Path to profiles JSON file")
			fs.StringVar(&cfg.HistoryFile, "history-file", cfg.HistoryFile, "Path to history JSONL file")
			if err := fs.Parse(retryArgs); err == nil && len(fs.Args()) == 0 {
				return cfg, nil
			}
		}
		return cfg, errors.New("usage: dbquery reset <config|profile|all>")
	}
	if len(rest) == 1 {
		cfg.ResetTarget = strings.ToLower(strings.TrimSpace(rest[0]))
	}

	switch cfg.ResetTarget {
	case "config", "profile", "all":
	default:
		return cfg, fmt.Errorf("unsupported reset target %q (expected config|profile|all)", cfg.ResetTarget)
	}

	return cfg, nil
}

func parseShowConfig(args []string) (Config, error) {
	cfg := Config{
		Mode:         modeShow,
		ShowTarget:   "all",
		SettingsFile: defaultSettingsFile(),
		ProfilesFile: defaultProfilesFile(),
	}

	fs := flag.NewFlagSet("dbquery show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.SettingsFile, "settings-file", cfg.SettingsFile, "Path to defaults settings JSON file")
	fs.StringVar(&cfg.ProfilesFile, "profiles-file", cfg.ProfilesFile, "Path to profiles JSON file")

	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage:\n")
		fmt.Fprintf(out, "  dbquery show [all|settings|profiles] [options]\n\n")
		fmt.Fprintf(out, "Examples:\n")
		fmt.Fprintf(out, "  dbquery show\n")
		fmt.Fprintf(out, "  dbquery show settings\n")
		fmt.Fprintf(out, "  dbquery show profiles\n\n")
		fmt.Fprintf(out, "Options:\n")
		fs.PrintDefaults()
	}

	parseArgs := args
	if len(args) > 0 && isShowTargetToken(args[0]) {
		cfg.ShowTarget = strings.ToLower(strings.TrimSpace(args[0]))
		parseArgs = args[1:]
	}

	if err := fs.Parse(parseArgs); err != nil {
		return cfg, err
	}

	rest := fs.Args()
	if len(rest) > 1 {
		return cfg, errors.New("usage: dbquery show [all|settings|profiles]")
	}
	if len(rest) == 1 {
		cfg.ShowTarget = strings.ToLower(strings.TrimSpace(rest[0]))
	}

	if !isShowTargetToken(cfg.ShowTarget) {
		return cfg, fmt.Errorf("unsupported show target %q (expected all|settings|profiles)", cfg.ShowTarget)
	}

	return cfg, nil
}

func isResetTargetToken(v string) bool {
	t := strings.ToLower(strings.TrimSpace(v))
	return t == "config" || t == "profile" || t == "all"
}

func isShowTargetToken(v string) bool {
	t := strings.ToLower(strings.TrimSpace(v))
	return t == "all" || t == "settings" || t == "profiles"
}

func removeFirstArg(args []string, needle string) []string {
	out := make([]string, 0, len(args))
	removed := false
	for _, a := range args {
		if !removed && a == needle {
			removed = true
			continue
		}
		out = append(out, a)
	}
	return out
}

func splitAndTrimCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func scanStringFlag(args []string, name string) (string, bool) {
	prefix := "--" + name + "="
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(a, prefix)), true
		}
		if a == "--"+name {
			if i+1 >= len(args) {
				return "", true
			}
			return strings.TrimSpace(args[i+1]), true
		}
	}
	return "", false
}

func printChatHelp() {
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  :help        Show help")
	fmt.Fprintln(os.Stderr, "  :exit        Exit chat mode")
	fmt.Fprintln(os.Stderr, "  :quit        Exit chat mode")
	fmt.Fprintln(os.Stderr, "Enter any other text to run it as a natural-language database query.")
}

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".dbquery"
	}
	return filepath.Join(home, ".dbquery")
}

func defaultHistoryFile() string {
	return filepath.Join(defaultConfigDir(), "history.jsonl")
}

func defaultProfilesFile() string {
	return filepath.Join(defaultConfigDir(), "profiles.json")
}

func defaultSettingsFile() string {
	return filepath.Join(defaultConfigDir(), "settings.json")
}

func normalizeDBTypeInput(v string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(v))
	switch t {
	case "sqlite", "postgres", "mysql":
		return t, nil
	case "postgresql", "pg":
		return "postgres", nil
	default:
		return "", fmt.Errorf("unsupported --db-type %q (expected sqlite|postgres|postgresql|mysql)", v)
	}
}

func detectDBTypeFromLocation(v string) (string, error) {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return "", errors.New("db url/path cannot be empty")
	}

	lower := strings.ToLower(raw)

	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return "postgres", nil
	}
	if strings.HasPrefix(lower, "mysql://") {
		return "mysql", nil
	}
	if strings.HasPrefix(lower, "sqlite://") || strings.HasPrefix(lower, "sqlite3://") || strings.HasPrefix(lower, "file:") {
		return "sqlite", nil
	}

	if strings.Contains(lower, "@tcp(") || strings.Contains(lower, "@unix(") {
		return "mysql", nil
	}

	if lower == ":memory:" || strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3") {
		return "sqlite", nil
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "~") {
		return "sqlite", nil
	}

	if strings.Contains(lower, "host=") && strings.Contains(lower, "user=") {
		return "postgres", nil
	}

	if parsed, err := url.Parse(raw); err == nil {
		scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
		switch scheme {
		case "postgres", "postgresql":
			return "postgres", nil
		case "mysql":
			return "mysql", nil
		case "sqlite", "sqlite3", "file":
			return "sqlite", nil
		}
	}

	return "", fmt.Errorf("unable to detect db type from %q; use explicit type: dbquery set db <sqlite|postgres|mysql> <db-url-or-path>", v)
}
