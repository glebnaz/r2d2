package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMainBuildAndRun(t *testing.T) {
	// Verify the binary builds successfully
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "r2d2")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = filepath.Join(findModuleRoot(t), "cmd", "r2d2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build: %s\n%s", err, out)
	}

	// Running without config should fail with an error
	cmd = exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+tmpDir)
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when running without config, got none")
	}

	// Running with a valid config should succeed
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{
		"vault_path": "/tmp/vault",
		"telegram_token": "test-token",
		"telegram_chat_id": 12345
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(binary, "--config", configPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success with valid config, got: %s\n%s", err, out)
	}
	if len(out) == 0 {
		t.Fatal("expected some output")
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
