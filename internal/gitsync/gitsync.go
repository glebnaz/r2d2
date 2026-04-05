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
	cmd := exec.CommandContext(ctx, "git", args...)
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
	start := time.Now()
	metrics.GitSyncsTotal.Inc()
	defer func() {
		metrics.GitSyncDuration.Observe(time.Since(start).Seconds())
	}()

	// Fetch latest remote state to keep origin/<branch> up to date.
	if _, stderr, err := s.git(ctx, "fetch", "origin", s.cfg.Branch); err != nil {
		return fmt.Errorf("git fetch: %s: %w", stderr, err)
	}

	// rsync vault to workDir, excluding .git and .obsidian.
	rsyncCmd := exec.CommandContext(ctx, "rsync",
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
	if _, stderr, err := s.git(ctx, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %s: %w", stderr, err)
	}

	// Check if there are staged changes.
	if _, _, err := s.git(ctx, "diff", "--cached", "--quiet"); err == nil {
		s.logger.Info("no changes to sync")
		metrics.GitFilesChanged.Set(0)
		return nil
	}

	// Get diff stat for commit message.
	diffStat, _, _ := s.git(ctx, "diff", "--cached", "--stat")

	// Count changed files.
	namesOut, _, _ := s.git(ctx, "diff", "--cached", "--name-only")
	filesChanged := countLines(namesOut)
	metrics.GitFilesChanged.Set(float64(filesChanged))

	// Commit.
	commitMsg := fmt.Sprintf("vault sync: %d files changed\n\n%s", filesChanged, strings.TrimSpace(diffStat))
	if _, stderr, err := s.git(ctx, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %s: %w", stderr, err)
	}

	// Push.
	_, pushStderr, pushErr := s.git(ctx, "push", "origin", s.cfg.Branch)
	if pushErr != nil {
		if isConflict(pushStderr) {
			metrics.GitConflicts.Inc()
			metrics.GitPushErrors.Inc()
			s.logger.Error("push conflict detected", "stderr", pushStderr)
			if s.cfg.NotifyOnConflict {
				msg := FormatConflictAlert(pushStderr, time.Now())
				if notifyErr := s.notifier.SendMessage(ctx, msg); notifyErr != nil {
					s.logger.Error("failed to send conflict notification", "error", notifyErr)
				}
			}
			// Reset local branch to match remote to prevent permanent divergence.
			if _, errOut, resetErr := s.git(ctx, "reset", "--hard", "origin/"+s.cfg.Branch); resetErr != nil {
				s.logger.Error("failed to reset after conflict", "error", resetErr, "stderr", errOut)
			}
			return fmt.Errorf("push conflict: %s", pushStderr)
		}
		metrics.GitPushErrors.Inc()
		return fmt.Errorf("git push: %s: %w", pushStderr, pushErr)
	}

	metrics.GitPushesTotal.Inc()

	s.logger.Info("push successful", "files_changed", filesChanged)
	if s.cfg.NotifyOnPush {
		msg := FormatPushNotification(filesChanged, diffStat, time.Now())
		if notifyErr := s.notifier.SendMessage(ctx, msg); notifyErr != nil {
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
