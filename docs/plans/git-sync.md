# Plan: Git Sync Obsidian Vault to GitHub

## Overview
Add a new feature to R2D2 that periodically pushes Obsidian vault changes to a GitHub repository. The bot copies files from the read-only vault mount into a separate git working tree via rsync, creates digestible commits, and pushes to GitHub. Push success and conflict events trigger Telegram notifications. The feature is fully configurable and follows existing project patterns (separate package under `internal/`, Prometheus metrics, Russian-language messages).

## Validation Commands
- `go test ./...`
- `go build -o r2d2 ./cmd/r2d2`
- `golangci-lint run`

### Task 1: Config — add GitSyncConfig
- [x] Add `GitSyncConfig` struct to `internal/config/config.go` with fields: `Enabled`, `RepoURL`, `Branch`, `WorkDir`, `PushIntervalMin`, `AuthorName`, `AuthorEmail`, `NotifyOnPush`, `NotifyOnConflict`
- [x] Add `GitSync *GitSyncConfig` field to `Config` struct (pointer, omitempty)
- [x] Add validation: if enabled, require `RepoURL` and `WorkDir`; set defaults for `Branch` ("main"), `PushIntervalMin` (30), `AuthorName` ("R2D2 Bot"), `AuthorEmail` ("r2d2@bot.local"), `NotifyOnPush` (true), `NotifyOnConflict` (true)
- [x] Add tests for new config fields (valid, missing required, defaults, disabled/omitted)
- [x] Mark completed

### Task 2: Metrics — add git sync Prometheus counters
- [x] Add metrics to `internal/metrics/metrics.go`: `r2d2_git_syncs_total`, `r2d2_git_pushes_total`, `r2d2_git_push_errors_total`, `r2d2_git_conflicts_total`, `r2d2_git_sync_duration_seconds` (histogram), `r2d2_git_files_changed_last` (gauge)
- [x] Register all new metrics in `init()`
- [x] Add tests
- [x] Mark completed

### Task 3: Core gitsync package — Syncer with Run/sync/ensureRepo
- [x] Create `internal/gitsync/gitsync.go` with `Notifier` interface, `Syncer` struct, and `New()` constructor
- [x] Implement `ensureRepo(ctx)`: clone if workDir empty, verify remote + fetch if exists
- [x] Implement `git(ctx, args...)` helper: exec git commands in workDir with author env vars
- [x] Implement `sync(ctx)`: rsync vault→workDir, check for changes, stage, build commit message from `git diff --cached --stat`, commit, push
- [x] Implement `Run(ctx)`: ensureRepo on startup, then ticker loop calling sync, return on ctx cancel
- [x] Handle push conflicts: detect "non-fast-forward"/"rejected" in stderr, send urgent notification, increment conflict metric
- [x] Instrument with Prometheus metrics (syncs total, pushes, errors, conflicts, duration, files changed)
- [x] Add tests (no changes skip, with changes commit, push conflict detection, notifications enabled/disabled, ensureRepo clone/existing)
- [x] Mark completed

### Task 4: Message formatting — Russian notifications with emoji
- [ ] Create `internal/gitsync/format.go` with `FormatPushNotification(filesChanged int, summary string, timestamp time.Time) string` — outputs `📤 Git Sync` message in Russian
- [ ] Add `FormatConflictAlert(stderr string, timestamp time.Time) string` — outputs `🚨 Git Sync — конфликт!` urgent message
- [ ] Add tests for both formatters
- [ ] Mark completed

### Task 5: Wire into main.go and update Dockerfile
- [ ] Add conditional gitsync startup in `cmd/r2d2/main.go`: if `cfg.GitSync != nil && cfg.GitSync.Enabled`, create Syncer and run in goroutine with shared ctx
- [ ] Add `git`, `rsync`, `openssh-client` to Dockerfile runtime image (`apk add --no-cache`)
- [ ] Add tests
- [ ] Mark completed

### Task 6: Documentation
- [ ] Update `CLAUDE.md`: add `internal/gitsync/` to architecture, new config fields, new metrics
- [ ] Update `docs/architecture/deployment.md`: add git-vault volume mount, GIT_TOKEN env, updated docker-compose example
- [ ] Create `docs/architecture/git-sync.md` describing the sync flow and design decisions
- [ ] Mark completed
