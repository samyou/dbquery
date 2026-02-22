# SKILL: Using `dbquery` CLI

This guide is for AI agents to reliably use the `dbquery` CLI.

## Purpose

`dbquery` converts natural-language questions into SQL (via LLM), executes against a DB, and renders results.

Supported DBs:
- `sqlite`
- `postgres`
- `mysql`

## Fast Setup

Set defaults once:

```bash
./dbquery set llm-key <API_KEY>
./dbquery set db sqlite ./examples/sample-sqlite.db
```

Or inline every run:

```bash
./dbquery --api-key <API_KEY> --db-type sqlite --db-url ./examples/sample-sqlite.db --query "latest users"
```

Notes:
- API key is read from `--api-key` or saved config (`dbquery set llm-key ...`).
- No environment-variable API key fallback.
- Quote DB URLs containing `?` (especially in zsh).

## Core Commands

One-shot query:

```bash
./dbquery --query "get all users created in the last 10 days"
```

Interactive mode:

```bash
./dbquery chat
```

History (JSON by default):

```bash
./dbquery history
```

Show saved config/profiles:

```bash
./dbquery show
./dbquery show settings
./dbquery show profiles
```

## Agent Workflow (Recommended)

1. Confirm defaults exist:

```bash
./dbquery show settings
```

2. For automation-friendly output, use JSON:

```bash
./dbquery --query "..." --output json
```

3. For safer SQL generation, preview first:

```bash
./dbquery --query "..." --dry-run --show-sql
```

4. Then execute without `--dry-run`.

## Useful Flags

- `--output table|json` (default query output is `table`)
- `--output-file <path>`
- `--limit <n>` (default `10`)
- `--show-sql`
- `--dry-run`
- `--tables users,orders` (scope schema)
- `--schema-file <file>` (extra context for LLM)
- `--profile <name>`

## Safety and Reset

- SQL is read-only by default.
- `--allow-write` allows write/DDL SQL.

Reset saved data:

```bash
./dbquery reset config
./dbquery reset profile
./dbquery reset all
./dbquery reset --dry-run all
./dbquery reset -y all
```

## Troubleshooting

- If sqlite path is invalid, CLI returns explicit path errors.
- If API key missing: set with `dbquery set llm-key ...` or pass `--api-key`.
- If DB type cannot be inferred in `set db`: provide explicit type.
