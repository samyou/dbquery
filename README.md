# dbquery

`dbquery` is a Go CLI that lets users query databases using natural language with an LLM.

Supported databases:
- SQLite
- PostgreSQL
- MySQL

It can:
- turn natural language into SQL
- run SQL safely (read-only by default)
- render output as terminal table or JSON
- write output to a file
- run in interactive chat mode
- save/load reusable profiles
- keep query history and display it later
- reset saved config/profile/history data

## Install

```bash
go mod tidy
go build -o dbquery ./cmd/dbquery
```

### Install via Homebrew tap

```bash
brew tap samyou/tap
brew install dbquery
```

Set API key once with `dbquery set` (recommended), or pass `--api-key` inline:

```bash
./dbquery set llm-key sk_12345
./dbquery set db sqlite ./app.db
./dbquery set db postgresql://user:pass@localhost:5432/app?sslmode=disable
```

## Quick Start

### Try with sample DB

```bash
./dbquery \
  --db-type sqlite \
  --db-url ./example/sample-sqlite.db \
  --query "get all rows in apple"
```

### SQLite

```bash
./dbquery \
  --db-type sqlite \
  --db-url ./app.db \
  --query "get all users created in the last 10 days"
```

### PostgreSQL

```bash
./dbquery \
  --db-type postgres \
  --db-url "postgres://user:password@localhost:5432/mydb?sslmode=disable" \
  --query "top 5 users by order count" \
  --output json
```

### MySQL

```bash
./dbquery \
  --db-type mysql \
  --db-url "user:password@tcp(localhost:3306)/mydb?parseTime=true" \
  --query "orders from last 30 days"
```

## Commands

### 1) One-shot query (default)

```bash
./dbquery --db-type sqlite --db-url ./app.db --query "latest users"
```

### 2) Interactive mode

```bash
./dbquery chat --db-type sqlite --db-url ./app.db
```

Interactive commands:
- `:help` show help
- `:exit` or `:quit` leave interactive mode

### 3) Show history

```bash
./dbquery history
```

By default, `history` prints JSON.

### 4) Save default credentials and DB

```bash
./dbquery set llm-key sk_12345
./dbquery set db postgresql://user:pass@localhost:5432/app?sslmode=disable
./dbquery set db sqlite ./example/sdsdf.db
./dbquery set db postgresql postgres postgres://user:pass@localhost:5432/app?sslmode=disable
```

`dbquery set db <db-url-or-path>` auto-detects db type when possible. You can still pass explicit db type, and that form also accepts an optional name token (`postgres` in the example).

### 5) Reset saved data

```bash
./dbquery reset config
./dbquery reset profile
./dbquery reset all
./dbquery reset --dry-run all
./dbquery reset -y
```

`reset` asks for confirmation (`[Y/n]`) unless `-y` is used.

## Query/Chat Options

These options apply to both default query mode and `chat` mode.

| Option | Type | Default | Description |
|---|---|---|---|
| `--db-type` | string | required unless saved/profiled | `sqlite`, `postgres`, or `mysql` |
| `--db-url` | string | required unless saved/profiled | DB URL/DSN, or sqlite file path |
| `--query` | string | required in one-shot mode | Natural language request |
| `--output` | string | `table` | Output format: `table` or `json` |
| `--output-file` | string | empty | Write rendered output to file |
| `--limit` | int | `10` | Default max rows |
| `--no-auto-limit` | bool | `false` | Do not auto-append `LIMIT` when missing |
| `--tables` | string | empty | Comma-separated table scope for schema/query generation |
| `--schema-file` | string | empty | Extra schema/business context file |
| `--schema-max-tables` | int | `40` | Max auto-discovered tables in prompt |
| `--model` | string | `gpt-4o-mini` | LLM model name (or `LLM_MODEL`) |
| `--api-key` | string | empty | API key override (falls back to saved config) |
| `--llm-base-url` | string | `https://api.openai.com/v1` | OpenAI-compatible endpoint |
| `--temperature` | float | `0` | LLM temperature |
| `--max-tokens` | int | `500` | LLM max completion tokens |
| `--timeout` | duration | `30s` | Timeout per query |
| `--show-sql` | bool | `false` | Print generated SQL |
| `--dry-run` | bool | `false` | Generate SQL only, do not execute |
| `--allow-write` | bool | `false` | Allow generated non-read-only SQL |
| `--verbose` | bool | `false` | Extra logs/warnings |
| `--history-file` | string | `~/.dbquery/history.jsonl` | History storage path |
| `--no-history` | bool | `false` | Disable history recording |
| `--profile` | string | empty | Load saved profile before applying flags |
| `--save-profile` | string | empty | Save current settings to a profile |
| `--profiles-file` | string | `~/.dbquery/profiles.json` | Profiles storage path |
| `--settings-file` | string | `~/.dbquery/settings.json` | Saved defaults file used by `dbquery set` |

## Set Command

Use `dbquery set` to store defaults for API key and DB.

```bash
./dbquery set llm-key sk_12345
./dbquery set db postgresql://user:pass@localhost:5432/app?sslmode=disable
./dbquery set db sqlite ./example/app.db
./dbquery set db postgresql postgres://user:pass@localhost:5432/app?sslmode=disable
./dbquery set --settings-file ./local-settings.json db mysql user:pass@tcp(localhost:3306)/mydb?parseTime=true
```

Supported keys:
- `llm-key` stores default API key
- `db` stores default DB type + DB URL/path (auto-detected from URL/path when type is omitted)

## Reset Command

Use `dbquery reset` to remove saved files.

```bash
./dbquery reset config
./dbquery reset profile
./dbquery reset all
./dbquery reset --dry-run all
./dbquery reset -y
```

Targets:
- `config`: remove saved defaults file (`settings.json`)
- `profile`: remove saved profiles file (`profiles.json`)
- `all`: remove config + profiles + history files

Options:
- `-y`: skip confirmation prompt
- `--dry-run`: show what would be deleted without deleting
- `--settings-file`: custom config file path for reset
- `--profiles-file`: custom profiles file path for reset
- `--history-file`: custom history file path for reset

### Override Priority

When values come from multiple places, precedence is:

1. inline CLI flags (highest)
2. selected profile (`--profile`)
3. saved defaults from `dbquery set`

## History Options

Use with `dbquery history`.

| Option | Type | Default | Description |
|---|---|---|---|
| `--history-file` | string | `~/.dbquery/history.jsonl` | History file to read |
| `--limit` | int | `20` | Number of recent entries to show |
| `--output` | string | `json` | `table` or `json` |
| `--full` | bool | `false` | Include SQL text in table output |

## Output Modes

### Table output

```bash
./dbquery --db-type sqlite --db-url ./app.db --query "latest users" --output table
```

### JSON output

```bash
./dbquery --db-type sqlite --db-url ./app.db --query "latest users" --output json
```

### Write output to file

```bash
./dbquery \
  --db-type sqlite \
  --db-url ./app.db \
  --query "latest users" \
  --output json \
  --output-file ./result.json
```

## Profiles

Profiles let you reuse DB + LLM settings.

### Save a profile

```bash
./dbquery \
  --db-type postgres \
  --db-url "postgres://user:password@localhost:5432/mydb?sslmode=disable" \
  --model gpt-4o-mini \
  --output table \
  --save-profile dev
```

### Use a profile (one-shot)

```bash
./dbquery --profile dev --query "users created this week"
```

### Use a profile (interactive)

```bash
./dbquery chat --profile dev
```

Flags passed on command line override loaded profile values.

## History

Each query (including `--dry-run`) is recorded in JSONL by default.

Show latest 50 entries as JSON:

```bash
./dbquery history --limit 50 --output json
```

Show latest 20 entries with SQL text:

```bash
./dbquery history --full
```

## Release (GoReleaser + Homebrew Tap)

This repository is configured to publish GitHub releases and update Homebrew formula in:
- CLI repo: `samyou/dbquery`
- Tap repo: `samyou/homebrew-tap`

### One-time setup

1. Add a GitHub token secret in `samyou/dbquery`:
   - name: `HOMEBREW_TAP_GITHUB_TOKEN`
   - scope: repo access to `samyou/homebrew-tap` (contents write)
2. Ensure the tap repo has `Formula/` directory.

### Release flow

```bash
git tag v0.1.0
git push origin v0.1.0
```

Tag push triggers `.github/workflows/release.yml`, which runs GoReleaser using `.goreleaser.yaml` to:
- build binaries for darwin/linux (amd64/arm64)
- create GitHub release assets + checksums
- update `Formula/dbquery.rb` in `samyou/homebrew-tap`

## Safety Notes

- By default, generated SQL must be read-only.
- `--allow-write` disables that safety check.
- `--limit` is automatically appended when query has no explicit limit (unless `--no-auto-limit`).
- Always verify generated SQL for production use.
