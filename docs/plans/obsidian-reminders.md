# Plan: Telegram bot for Obsidian task reminders

## Overview

Build a Go Telegram bot (r2d2) that reads tasks from an Obsidian vault, parses YAML frontmatter for `due` dates, and sends reminders via Telegram. Tasks with only a date (`YYYY-MM-DD`) trigger a reminder at 09:00; tasks with datetime (`YYYY-MM-DDTHH:mm`) trigger at the specified time. The bot runs on Raspberry Pi using long polling. Configuration (vault path, Telegram token, chat ID, timezone) is loaded from a JSON config file. Architecture is modular to support future features.

## Validation Commands

* `go test ./...`
* `golangci-lint run`
* `go build -o r2d2 ./cmd/r2d2`

### Task 1: Initialize project and CLI entry point

* [x] Create Go module `r2d2`
* [x] Create directories `cmd/r2d2`, `internal/config`, `internal/obsidian`, `internal/scheduler`, `internal/telegram`, `internal/reminder`
* [x] Implement `main.go` that loads config and starts the bot
* [x] Add tests
* [x] Mark completed

### Task 2: Implement JSON config loading

* [x] Define config struct with fields: `vault_path`, `telegram_token`, `telegram_chat_id`, `timezone` (default `Europe/Moscow`), `morning_hour` (default 9), `scan_interval_minutes` (default 5), `reminder_statuses` (default `["todo","in-progress","block"]`)
* [x] Set default config path `~/.config/r2d2/config.json`
* [x] Accept config path override via `--config` flag
* [x] Validate required fields (`vault_path`, `telegram_token`, `telegram_chat_id`)
* [x] Add tests
* [x] Mark completed

### Task 3: Implement Obsidian vault scanner

* [x] Recursively scan `vault_path` for `.md` files
* [x] Parse YAML frontmatter between `---` delimiters using `gopkg.in/yaml.v3`
* [x] Extract fields: `type`, `status`, `due`, `priority`, `project`
* [x] Filter: only `type: Task`, status in configured `reminder_statuses`
* [x] Filter: only tasks where `due` is set
* [x] Parse `due` as `YYYY-MM-DD` (date-only) or `YYYY-MM-DDTHH:mm` (datetime)
* [x] Return list of `Task` structs with: title (from filename), due (time.Time), has_time (bool), priority, project, file_path
* [x] Add tests with sample frontmatter files
* [x] Mark completed

### Task 4: Implement Telegram client

* [x] Use `go-telegram-bot-api/telegram-bot-api` library with long polling
* [x] Implement `SendMessage(chatID, text)` with Markdown formatting
* [x] Implement bot startup and graceful shutdown
* [x] Handle connection errors with retry and logging
* [x] Add tests using mock HTTP server
* [x] Mark completed

### Task 5: Implement reminder scheduler

* [x] On startup, scan vault and build schedule for today's tasks
* [x] For date-only tasks: schedule reminder at `morning_hour` (default 09:00)
* [x] For datetime tasks: schedule reminder at the specified time
* [x] Skip tasks whose reminder time has already passed today
* [x] Re-scan vault periodically (every `scan_interval_minutes`) to pick up new/changed tasks
* [x] At midnight, rebuild the full schedule for the new day
* [x] Track sent reminders (in-memory set by file_path + date) to avoid duplicates
* [x] Add tests
* [x] Mark completed

### Task 6: Format reminder messages

* [ ] Morning digest: group all date-only tasks due today into one message, sorted by priority (high first)
* [ ] Include task title, priority (if set), project (if set)
* [ ] Timed reminders: send individual message per task at its scheduled time
* [ ] Overdue tasks: include in morning digest with "overdue" label if `due < today`
* [ ] Use Telegram Markdown formatting for readability
* [ ] Add tests
* [ ] Mark completed

### Task 7: Wire everything together and add graceful shutdown

* [ ] Connect scanner, scheduler, telegram client in `main.go`
* [ ] Handle OS signals (SIGINT, SIGTERM) for graceful shutdown
* [ ] Add structured logging (slog)
* [ ] Add `--dry-run` flag that prints reminders to stdout instead of Telegram
* [ ] Test end-to-end with sample vault
* [ ] Mark completed
