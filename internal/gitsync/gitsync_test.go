package gitsync

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"r2d2/internal/config"
)

type mockNotifier struct {
	messages []string
}

func (m *mockNotifier) SendMessage(_ context.Context, text string) error {
	m.messages = append(m.messages, text)
	return nil
}

// requireTools skips the test if git or rsync are not available.
func requireTools(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not found")
	}
}

// runCmd executes a command and fails the test on error.
func runCmd(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

// setupBareRepo creates a bare git repo with an initial commit on "main" and returns its path.
func setupBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bare := filepath.Join(dir, "remote.git")
	runCmd(t, "", "git", "init", "--bare", bare)

	// Create initial commit via a temp working copy.
	tmp := filepath.Join(dir, "init-work")
	os.MkdirAll(tmp, 0o755)
	runCmd(t, tmp, "git", "init")
	runCmd(t, tmp, "git", "remote", "add", "origin", bare)
	runCmd(t, tmp, "git", "checkout", "-b", "main")
	runCmd(t, tmp, "git", "config", "user.email", "test@test.local")
	runCmd(t, tmp, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(tmp, ".gitkeep"), []byte(""), 0o644)
	runCmd(t, tmp, "git", "add", "-A")
	runCmd(t, tmp, "git", "commit", "-m", "initial")
	runCmd(t, tmp, "git", "push", "-u", "origin", "main")

	return bare
}

// newTestSyncer creates a Syncer with a fresh workDir (not yet created) for testing.
func newTestSyncer(t *testing.T, bareRepo, vaultPath string, notifier *mockNotifier) *Syncer {
	t.Helper()
	workDir := filepath.Join(t.TempDir(), "work")

	cfg := &config.GitSyncConfig{
		Enabled:          true,
		RepoURL:          bareRepo,
		Branch:           "main",
		WorkDir:          workDir,
		PushIntervalMin:  1,
		AuthorName:       "Test Bot",
		AuthorEmail:      "test@bot.local",
		NotifyOnPush:     true,
		NotifyOnConflict: true,
	}

	return New(cfg, vaultPath, notifier, slog.Default())
}

func TestEnsureRepo_Clone(t *testing.T) {
	requireTools(t)
	bare := setupBareRepo(t)
	vault := t.TempDir()
	notifier := &mockNotifier{}
	syncer := newTestSyncer(t, bare, vault, notifier)

	ctx := context.Background()
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatalf("ensureRepo clone failed: %v", err)
	}

	// Verify .git directory exists in workDir.
	gitDir := filepath.Join(syncer.cfg.WorkDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Fatal(".git directory not created after clone")
	}

	// Verify the remote URL is correct.
	out := runCmd(t, syncer.cfg.WorkDir, "git", "remote", "get-url", "origin")
	if strings.TrimSpace(out) != bare {
		t.Fatalf("remote URL = %q, want %q", strings.TrimSpace(out), bare)
	}
}

func TestEnsureRepo_Existing(t *testing.T) {
	requireTools(t)
	bare := setupBareRepo(t)
	vault := t.TempDir()
	notifier := &mockNotifier{}
	syncer := newTestSyncer(t, bare, vault, notifier)

	ctx := context.Background()

	// First call clones.
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatalf("first ensureRepo failed: %v", err)
	}

	// Second call should fetch without error.
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatalf("second ensureRepo (fetch) failed: %v", err)
	}
}

func TestSync_NoChanges(t *testing.T) {
	requireTools(t)
	bare := setupBareRepo(t)

	// Set up vault with same content as repo.
	vault := t.TempDir()
	os.WriteFile(filepath.Join(vault, ".gitkeep"), []byte(""), 0o644)

	notifier := &mockNotifier{}
	syncer := newTestSyncer(t, bare, vault, notifier)

	ctx := context.Background()
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatal(err)
	}

	// Sync with no changes should succeed with no commit.
	if err := syncer.sync(ctx); err != nil {
		t.Fatalf("sync with no changes failed: %v", err)
	}

	// Verify no new commit (only "initial" commit).
	out := runCmd(t, syncer.cfg.WorkDir, "git", "log", "--oneline")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 commit, got %d: %s", len(lines), out)
	}

	// No notification should be sent.
	if len(notifier.messages) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(notifier.messages))
	}
}

func TestSync_WithChanges(t *testing.T) {
	requireTools(t)
	bare := setupBareRepo(t)

	vault := t.TempDir()
	os.WriteFile(filepath.Join(vault, ".gitkeep"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(vault, "notes.md"), []byte("# My Notes\n"), 0o644)

	notifier := &mockNotifier{}
	syncer := newTestSyncer(t, bare, vault, notifier)

	ctx := context.Background()
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatal(err)
	}

	if err := syncer.sync(ctx); err != nil {
		t.Fatalf("sync with changes failed: %v", err)
	}

	// Verify a new commit was created.
	out := runCmd(t, syncer.cfg.WorkDir, "git", "log", "--oneline")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 commits, got %d: %s", len(lines), out)
	}

	// Verify commit message mentions vault sync.
	logOut := runCmd(t, syncer.cfg.WorkDir, "git", "log", "-1", "--format=%s")
	if !strings.Contains(logOut, "vault sync") {
		t.Fatalf("commit message missing 'vault sync': %s", logOut)
	}

	// Verify push notification was sent.
	if len(notifier.messages) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.messages))
	}
	if !strings.Contains(notifier.messages[0], "Git Sync") {
		t.Errorf("notification missing 'Git Sync': %s", notifier.messages[0])
	}

	// Verify file was pushed to remote.
	verifyDir := filepath.Join(t.TempDir(), "verify")
	runCmd(t, "", "git", "clone", bare, verifyDir)
	if _, err := os.Stat(filepath.Join(verifyDir, "notes.md")); os.IsNotExist(err) {
		t.Fatal("notes.md not pushed to remote")
	}
}

func TestSync_PushConflict(t *testing.T) {
	requireTools(t)
	bare := setupBareRepo(t)

	vault := t.TempDir()
	os.WriteFile(filepath.Join(vault, ".gitkeep"), []byte(""), 0o644)

	notifier := &mockNotifier{}
	syncer := newTestSyncer(t, bare, vault, notifier)

	ctx := context.Background()
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatal(err)
	}

	// Push a conflicting change from another clone.
	conflictDir := filepath.Join(t.TempDir(), "conflict")
	runCmd(t, "", "git", "clone", bare, conflictDir)
	runCmd(t, conflictDir, "git", "config", "user.email", "other@test.local")
	runCmd(t, conflictDir, "git", "config", "user.name", "Other")
	os.WriteFile(filepath.Join(conflictDir, "conflict.txt"), []byte("conflict"), 0o644)
	runCmd(t, conflictDir, "git", "add", "-A")
	runCmd(t, conflictDir, "git", "commit", "-m", "conflicting change")
	runCmd(t, conflictDir, "git", "push", "origin", "main")

	// Add a file to vault so sync has something to commit.
	os.WriteFile(filepath.Join(vault, "new.md"), []byte("# New\n"), 0o644)

	// Sync should detect push conflict.
	err := syncer.sync(ctx)
	if err == nil {
		t.Fatal("expected push conflict error")
	}
	if !strings.Contains(err.Error(), "push conflict") {
		t.Fatalf("expected 'push conflict' in error, got: %v", err)
	}

	// Conflict notification should have been sent.
	if len(notifier.messages) != 1 {
		t.Fatalf("expected 1 conflict notification, got %d", len(notifier.messages))
	}
	if !strings.Contains(notifier.messages[0], "конфликт") {
		t.Errorf("notification missing 'конфликт': %s", notifier.messages[0])
	}
}

func TestSync_NotificationsDisabled(t *testing.T) {
	requireTools(t)
	bare := setupBareRepo(t)

	vault := t.TempDir()
	os.WriteFile(filepath.Join(vault, ".gitkeep"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(vault, "file.md"), []byte("content"), 0o644)

	notifier := &mockNotifier{}
	workDir := filepath.Join(t.TempDir(), "work")

	cfg := &config.GitSyncConfig{
		Enabled:          true,
		RepoURL:          bare,
		Branch:           "main",
		WorkDir:          workDir,
		PushIntervalMin:  1,
		AuthorName:       "Test Bot",
		AuthorEmail:      "test@bot.local",
		NotifyOnPush:     false,
		NotifyOnConflict: false,
	}
	syncer := New(cfg, vault, notifier, slog.Default())

	ctx := context.Background()
	if err := syncer.ensureRepo(ctx); err != nil {
		t.Fatal(err)
	}

	if err := syncer.sync(ctx); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// No notifications should be sent when disabled.
	if len(notifier.messages) != 0 {
		t.Fatalf("expected 0 notifications with notify disabled, got %d", len(notifier.messages))
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		stderr string
		want   bool
	}{
		{"error: failed to push some refs: non-fast-forward", true},
		{"! [rejected] main -> main (fetch first)", true},
		{"Everything up-to-date", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isConflict(tt.stderr)
		if got != tt.want {
			t.Errorf("isConflict(%q) = %v, want %v", tt.stderr, got, tt.want)
		}
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"  \n  ", 0},
		{"file.md", 1},
		{"a.md\nb.md\nc.md", 3},
		{"a.md\nb.md\n", 2},
	}
	for _, tt := range tests {
		got := countLines(tt.input)
		if got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
