package gitsync

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"r2d2/internal/config"
	"r2d2/internal/metrics"
)

const syncTimeout = 5 * time.Minute

// Notifier sends notifications to the user.
type Notifier interface {
	SendMessage(ctx context.Context, text string) error
}

// Syncer handles periodic git sync of an Obsidian vault.
type Syncer struct {
	cfg       *config.GitSyncConfig
	vaultPath string
	notifier  Notifier
	logger    *slog.Logger
}

// New creates a new Syncer.
func New(cfg *config.GitSyncConfig, vaultPath string, notifier Notifier, logger *slog.Logger) *Syncer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Syncer{
		cfg:       cfg,
		vaultPath: vaultPath,
		notifier:  notifier,
		logger:    logger,
	}
}

// gitEnv returns the environment variables for git commands.
func (s *Syncer) gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME="+s.cfg.AuthorName,
		"GIT_AUTHOR_EMAIL="+s.cfg.AuthorEmail,
		"GIT_COMMITTER_NAME="+s.cfg.AuthorName,
		"GIT_COMMITTER_EMAIL="+s.cfg.AuthorEmail,
	)
}

// git runs a git command in the work directory with configured author env vars.
func (s *Syncer) git(ctx context.Context, args ...string) (string, string, error) {
	// Prepend -c core.quotePath=false so git doesn't escape non-ASCII paths.
	full := append([]string{"-c", "core.quotePath=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = s.cfg.WorkDir
	cmd.Env = s.gitEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// ensureRepo clones the repository if the work directory has no .git,
// or verifies the remote and fetches if it already exists.
func (s *Syncer) ensureRepo(ctx context.Context) error {
	gitDir := filepath.Join(s.cfg.WorkDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("checking git directory: %w", err)
		}
		s.logger.Info("cloning repository", "url", s.cfg.RepoURL, "branch", s.cfg.Branch)
		cmd := exec.CommandContext(ctx, "git", "clone", "--branch", s.cfg.Branch, s.cfg.RepoURL, s.cfg.WorkDir)
		cmd.Env = s.gitEnv()
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			os.RemoveAll(s.cfg.WorkDir)
			return fmt.Errorf("cloning repo: %s: %w", stderr.String(), err)
		}
		return nil
	}

	// Verify remote URL matches config.
	stdout, stderr, err := s.git(ctx, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("getting remote URL: %s: %w", stderr, err)
	}
	remoteURL := strings.TrimSpace(stdout)
	if remoteURL != s.cfg.RepoURL {
		s.logger.Warn("remote URL mismatch, updating",
			"current", remoteURL,
			"expected", s.cfg.RepoURL,
		)
		if _, errOut, err := s.git(ctx, "remote", "set-url", "origin", s.cfg.RepoURL); err != nil {
			return fmt.Errorf("setting remote URL: %s: %w", errOut, err)
		}
	}

	s.logger.Info("fetching from remote", "branch", s.cfg.Branch)
	if _, errOut, err := s.git(ctx, "fetch", "origin", s.cfg.Branch); err != nil {
		return fmt.Errorf("fetching: %s: %w", errOut, err)
	}

	return nil
}

// sync performs one sync cycle: fetch, rsync vault to workDir, stage, commit, push.
func (s *Syncer) sync(ctx context.Context) error {
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	start := time.Now()
	metrics.GitSyncsTotal.Inc()
	defer func() {
		metrics.GitSyncDuration.Observe(time.Since(start).Seconds())
	}()

	// Fetch latest remote state to keep origin/<branch> up to date.
	if _, stderr, err := s.git(syncCtx, "fetch", "origin", s.cfg.Branch); err != nil {
		return fmt.Errorf("git fetch: %s: %w", stderr, err)
	}

	// Verify vault path exists and is non-empty to prevent rsync --delete from wiping the working tree.
	if _, err := os.Stat(s.vaultPath); err != nil {
		return fmt.Errorf("vault path %q not accessible: %w", s.vaultPath, err)
	}
	entries, err := os.ReadDir(s.vaultPath)
	if err != nil {
		return fmt.Errorf("reading vault path %q: %w", s.vaultPath, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("vault path %q is empty, skipping sync to prevent data loss", s.vaultPath)
	}

	// rsync vault to workDir, excluding .git and .obsidian.
	rsyncCmd := exec.CommandContext(syncCtx, "rsync",
		"-a", "--delete",
		"--exclude", ".git",
		"--exclude", ".obsidian",
		s.vaultPath+"/",
		s.cfg.WorkDir+"/",
	)
	var rsyncStderr bytes.Buffer
	rsyncCmd.Stderr = &rsyncStderr
	if err := rsyncCmd.Run(); err != nil {
		return fmt.Errorf("rsync: %s: %w", rsyncStderr.String(), err)
	}

	// Stage all changes.
	if _, stderr, err := s.git(syncCtx, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %s: %w", stderr, err)
	}

	// Commit if there are staged changes.
	if _, _, err := s.git(syncCtx, "diff", "--cached", "--quiet"); err != nil {
		diffStat, _, _ := s.git(syncCtx, "diff", "--cached", "--stat")
		namesOut, _, _ := s.git(syncCtx, "diff", "--cached", "--name-only")
		filesChanged := countLines(namesOut)
		metrics.GitFilesChanged.Set(float64(filesChanged))

		commitMsg := fmt.Sprintf("vault sync: %d files changed\n\n%s", filesChanged, strings.TrimSpace(diffStat))
		if _, stderr, err := s.git(syncCtx, "commit", "-m", commitMsg); err != nil {
			return fmt.Errorf("git commit: %s: %w", stderr, err)
		}
	} else {
		s.logger.Info("no new vault changes")
		metrics.GitFilesChanged.Set(0)
	}

	// Check if local branch is ahead of remote (handles new commits and retries of previously failed pushes).
	aheadOut, revListStderr, revListErr := s.git(syncCtx, "rev-list", fmt.Sprintf("origin/%s..HEAD", s.cfg.Branch), "--count")
	if revListErr != nil {
		return fmt.Errorf("git rev-list: %s: %w", revListStderr, revListErr)
	}
	if strings.TrimSpace(aheadOut) == "0" {
		return nil
	}

	// Collect changed file names for notification before pushing.
	pushNames, _, _ := s.git(syncCtx, "diff", "--name-only", fmt.Sprintf("origin/%s..HEAD", s.cfg.Branch))
	pushFilesChanged := countLines(pushNames)

	// Push.
	_, pushStderr, pushErr := s.git(syncCtx, "push", "origin", s.cfg.Branch)
	if pushErr != nil {
		if isConflict(pushStderr) {
			metrics.GitConflicts.Inc()
			metrics.GitPushErrors.Inc()
			s.logger.Error("push conflict detected", "stderr", pushStderr)
			if s.cfg.NotifyOnConflict {
				msg := FormatConflictAlert(pushStderr, time.Now())
				if notifyErr := s.notifier.SendMessage(syncCtx, msg); notifyErr != nil {
					s.logger.Error("failed to send conflict notification", "error", notifyErr)
				}
			}
			// Reset local branch to match remote to prevent permanent divergence.
			if _, errOut, resetErr := s.git(syncCtx, "reset", "--hard", "origin/"+s.cfg.Branch); resetErr != nil {
				s.logger.Error("failed to reset after conflict", "error", resetErr, "stderr", errOut)
			}
			return fmt.Errorf("push conflict: %s", pushStderr)
		}
		metrics.GitPushErrors.Inc()
		return fmt.Errorf("git push: %s: %w", pushStderr, pushErr)
	}

	metrics.GitPushesTotal.Inc()

	s.logger.Info("push successful", "files_changed", pushFilesChanged)
	if s.cfg.NotifyOnPush {
		msg := FormatPushNotification(pushFilesChanged, pushNames, time.Now())
		if notifyErr := s.notifier.SendMessage(syncCtx, msg); notifyErr != nil {
			s.logger.Error("failed to send push notification", "error", notifyErr)
		}
	}

	return nil
}

// Run starts the sync loop. It blocks until the context is cancelled.
func (s *Syncer) Run(ctx context.Context) error {
	s.logger.Info("git sync starting",
		"repo", s.cfg.RepoURL,
		"branch", s.cfg.Branch,
		"interval_min", s.cfg.PushIntervalMin,
	)

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := s.ensureRepo(ctx); err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("ensuring repo after %d attempts: %w", maxRetries, err)
			}
			s.logger.Warn("ensureRepo failed, retrying", "attempt", attempt, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt*10) * time.Second):
			}
			continue
		}
		break
	}

	if err := s.sync(ctx); err != nil {
		s.logger.Error("initial sync failed", "error", err)
	}

	ticker := time.NewTicker(time.Duration(s.cfg.PushIntervalMin) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("git sync stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := s.sync(ctx); err != nil {
				s.logger.Error("sync failed", "error", err)
			}
		}
	}
}

// isConflict checks if push stderr indicates a non-fast-forward conflict.
func isConflict(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "non-fast-forward") || strings.Contains(lower, "rejected")
}

// countLines counts non-empty lines in s.
func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
