package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMainBuildAndRun(t *testing.T) {
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "r2d2")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = filepath.Join(findModuleRoot(t), "cmd", "r2d2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build: %s\n%s", err, out)
	}

	// Running without config should fail.
	cmd = exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+tmpDir)
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when running without config, got none")
	}
}

func TestDryRunWithSampleVault(t *testing.T) {
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "r2d2")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = filepath.Join(findModuleRoot(t), "cmd", "r2d2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build: %s\n%s", err, out)
	}

	// Create sample vault with a task due today.
	vaultDir := filepath.Join(tmpDir, "vault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	taskContent := "---\ntype: Task\nstatus: todo\ndue: " + today + "\npriority: high\nproject: test\n---\n\nSample task body.\n"
	if err := os.WriteFile(filepath.Join(vaultDir, "Sample Task.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create config.
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
		"vault_path": "` + vaultDir + `",
		"telegram_token": "fake-token",
		"telegram_chat_id": 12345,
		"scan_interval_minutes": 1
	}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run in dry-run mode with a timeout so it doesn't block forever.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd = exec.CommandContext(ctx, binary, "--config", configPath, "--dry-run")
	out, _ = cmd.CombinedOutput()
	output := string(out)

	// The bot should start and log startup message. The scheduler runs
	// continuously, so we expect it to be killed by the timeout.
	if !strings.Contains(output, "r2d2 starting") && !strings.Contains(output, "DRY RUN") && !strings.Contains(output, "scheduler starting") {
		t.Errorf("expected startup or dry-run output, got: %s", output)
	}
}

func TestDryRunWithGitSyncEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "r2d2")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = filepath.Join(findModuleRoot(t), "cmd", "r2d2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build: %s\n%s", err, out)
	}

	// Create sample vault.
	vaultDir := filepath.Join(tmpDir, "vault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	taskContent := "---\ntype: Task\nstatus: todo\ndue: " + today + "\npriority: high\nproject: test\n---\n\nSample task.\n"
	if err := os.WriteFile(filepath.Join(vaultDir, "Task.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create config with git_sync enabled pointing to a non-existent repo.
	// The bot should start, log the git sync error, but not crash.
	gitWorkDir := filepath.Join(tmpDir, "git-work")
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
		"vault_path": "` + vaultDir + `",
		"telegram_token": "fake-token",
		"telegram_chat_id": 12345,
		"scan_interval_minutes": 1,
		"git_sync": {
			"enabled": true,
			"repo_url": "` + filepath.Join(tmpDir, "fake-repo.git") + `",
			"work_dir": "` + gitWorkDir + `",
			"branch": "main",
			"push_interval_min": 1
		}
	}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd = exec.CommandContext(ctx, binary, "--config", configPath, "--dry-run")
	out, _ = cmd.CombinedOutput()
	output := string(out)

	// The bot should start successfully even though git sync will fail to clone.
	if !strings.Contains(output, "r2d2 starting") && !strings.Contains(output, "scheduler starting") {
		t.Errorf("expected startup log, got: %s", output)
	}
}

func TestDryRunSender(t *testing.T) {
	sender := &dryRunSender{}
	err := sender.SendMessage(context.Background(), "hello test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root")
		}
		dir = parent
	}
}
