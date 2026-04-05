# R2D2 - Personal Telegram Bot

## Project overview

Personal Telegram bot (Go) running on Raspberry Pi. Extensible architecture — features are added as modules. Currently implemented: Obsidian task reminders, Obsidian vault git sync.

## Architecture

```
cmd/r2d2/main.go          — entry point, wires components, starts metrics server
internal/config/           — JSON config loading and validation
internal/obsidian/         — vault scanner, YAML frontmatter parser
internal/telegram/         — Telegram Bot API client with retry
internal/scheduler/        — reminder scheduling, dedup, midnight reset
internal/digest/           — collector-based morning digest engine
internal/reminder/         — message formatting (timed reminders)
internal/gitsync/          — periodic vault-to-GitHub sync via git+rsync
internal/metrics/          — Prometheus metrics
```

## Key design decisions

- **Digest engine with collectors**: `digest.Collector` interface allows adding new sections to the morning digest. Register new collectors in `main.go` via `engine.Register()`.
- **Scheduler owns timing**: All reminder scheduling (morning digest, timed tasks) lives in `scheduler.go`. Dedup via in-memory `sent` map, reset at midnight.
- **Scanner is stateless**: `obsidian.ScanVault()` does a full walk every time. No caching, no file watchers. Scan interval is configurable.
- **Formatting is in Russian**: All user-facing messages use Russian language and emoji.
- **Extensible by design**: New features (interactive commands, agent integrations, etc.) should be added as separate packages under `internal/`.
- **Git sync is independent**: `gitsync.Syncer` runs in its own goroutine with a ticker loop. Uses rsync to copy vault into a separate git working tree, then commits and pushes. Conflict detection triggers urgent Telegram notifications.

## Build and test

```bash
go test ./...
go build -o r2d2 ./cmd/r2d2
golangci-lint run
```

## Docker

```bash
docker build -t r2d2 .
# Multi-arch build happens via GH Actions (.github/workflows/docker.yml)
# Image: ghcr.io/glebnaz/r2d2:master
```

## Run locally

```bash
./r2d2 --config path/to/config.json
./r2d2 --config path/to/config.json --dry-run  # prints to stdout
```

## Config format

```json
{
  "vault_path": "/path/to/vault",
  "telegram_token": "BOT_TOKEN",
  "telegram_chat_id": 123456789,
  "timezone": "Europe/Berlin",
  "morning_hour": 9,
  "scan_interval_minutes": 5,
  "reminder_statuses": ["todo", "in-progress", "block"],
  "git_sync": {
    "enabled": true,
    "repo_url": "https://github.com/user/vault.git",
    "work_dir": "/data/git-vault",
    "branch": "main",
    "push_interval_min": 30,
    "author_name": "R2D2 Bot",
    "author_email": "r2d2@bot.local",
    "notify_on_push": true,
    "notify_on_conflict": true
  }
}
```

## Obsidian task format

Tasks are `.md` files with YAML frontmatter:
- `type: Task` (required)
- `status: todo|in-progress|block` (must match `reminder_statuses`)
- `due: YYYY-MM-DD` (date-only, reminder at morning hour) or `due: YYYY-MM-DDTHH:mm` (timed reminder)
- `priority: high|low` (optional)
- `project: "[[01 Name]]"` (optional, wikilink syntax is cleaned automatically)
- Description is extracted from `## Описание` section in the note body

## Adding a new digest collector

1. Create a struct implementing `digest.Collector` in `internal/digest/`
2. `Collect(ctx, now)` returns `*Section{Header, Body}` or nil to skip
3. Register in `main.go`: `engine.Register(myCollector)`

## Metrics

Prometheus metrics on `:9182/metrics`. Key metrics:
- `r2d2_tasks_scanned_total`, `r2d2_tasks_due_today`, `r2d2_tasks_overdue`
- `r2d2_digests_sent_total`, `r2d2_timed_reminders_sent_total`
- `r2d2_notification_errors_total`, `r2d2_vault_scan_errors_total`
- `r2d2_vault_scan_duration_seconds` (histogram)
- `r2d2_git_syncs_total`, `r2d2_git_pushes_total`, `r2d2_git_push_errors_total`
- `r2d2_git_conflicts_total`, `r2d2_git_sync_duration_seconds` (histogram)
- `r2d2_git_files_changed_last` (gauge)
- Standard Go runtime metrics (`go_*`, `process_*`)

## Deployment

Runs as Docker container on Raspberry Pi (arm64). Defined in `home-server` repo's `docker-compose.yaml`. Vault is mounted read-only from Obsidian Headless sync directory. Git sync requires a `r2d2-git-vault` Docker volume for the working tree. The Docker image includes `git`, `rsync`, and `openssh-client`.
