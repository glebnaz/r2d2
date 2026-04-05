package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{
		"vault_path": "/home/user/vault",
		"telegram_token": "123:ABC",
		"telegram_chat_id": 99999
	}`))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.VaultPath != "/home/user/vault" {
		t.Errorf("vault_path = %q, want /home/user/vault", cfg.VaultPath)
	}
	if cfg.Timezone != "Europe/Moscow" {
		t.Errorf("timezone = %q, want Europe/Moscow", cfg.Timezone)
	}
	if cfg.MorningHour != 9 {
		t.Errorf("morning_hour = %d, want 9", cfg.MorningHour)
	}
	if cfg.ScanIntervalMinutes != 5 {
		t.Errorf("scan_interval_minutes = %d, want 5", cfg.ScanIntervalMinutes)
	}
	if len(cfg.ReminderStatuses) != 3 {
		t.Errorf("reminder_statuses length = %d, want 3", len(cfg.ReminderStatuses))
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{
		"vault_path": "/vault",
		"telegram_token": "tok",
		"telegram_chat_id": 1,
		"timezone": "UTC",
		"morning_hour": 8,
		"scan_interval_minutes": 10,
		"reminder_statuses": ["todo"]
	}`))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", cfg.Timezone)
	}
	if cfg.MorningHour != 8 {
		t.Errorf("morning_hour = %d, want 8", cfg.MorningHour)
	}
	if cfg.ScanIntervalMinutes != 10 {
		t.Errorf("scan_interval_minutes = %d, want 10", cfg.ScanIntervalMinutes)
	}
	if len(cfg.ReminderStatuses) != 1 {
		t.Errorf("reminder_statuses length = %d, want 1", len(cfg.ReminderStatuses))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"missing vault_path", `{"telegram_token":"t","telegram_chat_id":1}`},
		{"missing telegram_token", `{"vault_path":"/v","telegram_chat_id":1}`},
		{"missing telegram_chat_id", `{"vault_path":"/v","telegram_token":"t"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			writeFile(t, path, []byte(tt.json))

			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error for missing required field")
			}
		})
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`not json`))

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGitSyncDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{
		"vault_path": "/vault",
		"telegram_token": "tok",
		"telegram_chat_id": 1,
		"git_sync": {
			"enabled": true,
			"repo_url": "https://github.com/user/repo.git",
			"work_dir": "/tmp/sync"
		}
	}`))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs := cfg.GitSync
	if gs == nil {
		t.Fatal("git_sync should not be nil")
	}
	if gs.Branch != "main" {
		t.Errorf("branch = %q, want main", gs.Branch)
	}
	if gs.PushIntervalMin != 30 {
		t.Errorf("push_interval_min = %d, want 30", gs.PushIntervalMin)
	}
	if gs.AuthorName != "R2D2 Bot" {
		t.Errorf("author_name = %q, want R2D2 Bot", gs.AuthorName)
	}
	if gs.AuthorEmail != "r2d2@bot.local" {
		t.Errorf("author_email = %q, want r2d2@bot.local", gs.AuthorEmail)
	}
	if !gs.NotifyOnPush {
		t.Error("notify_on_push should default to true")
	}
	if !gs.NotifyOnConflict {
		t.Error("notify_on_conflict should default to true")
	}
}

func TestGitSyncOverrideDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{
		"vault_path": "/vault",
		"telegram_token": "tok",
		"telegram_chat_id": 1,
		"git_sync": {
			"enabled": true,
			"repo_url": "https://github.com/user/repo.git",
			"work_dir": "/tmp/sync",
			"branch": "master",
			"push_interval_min": 15,
			"author_name": "Custom",
			"author_email": "custom@example.com",
			"notify_on_push": false,
			"notify_on_conflict": false
		}
	}`))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs := cfg.GitSync
	if gs.Branch != "master" {
		t.Errorf("branch = %q, want master", gs.Branch)
	}
	if gs.PushIntervalMin != 15 {
		t.Errorf("push_interval_min = %d, want 15", gs.PushIntervalMin)
	}
	if gs.AuthorName != "Custom" {
		t.Errorf("author_name = %q, want Custom", gs.AuthorName)
	}
	if gs.AuthorEmail != "custom@example.com" {
		t.Errorf("author_email = %q, want custom@example.com", gs.AuthorEmail)
	}
	if gs.NotifyOnPush {
		t.Error("notify_on_push should be false")
	}
	if gs.NotifyOnConflict {
		t.Error("notify_on_conflict should be false")
	}
}

func TestGitSyncMissingRequired(t *testing.T) {
	base := `"vault_path":"/v","telegram_token":"t","telegram_chat_id":1`
	tests := []struct {
		name string
		json string
	}{
		{"missing repo_url", `{` + base + `,"git_sync":{"enabled":true,"work_dir":"/tmp/sync"}}`},
		{"missing work_dir", `{` + base + `,"git_sync":{"enabled":true,"repo_url":"https://github.com/u/r.git"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			writeFile(t, path, []byte(tt.json))
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error for missing git_sync required field")
			}
		})
	}
}

func TestGitSyncDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{
		"vault_path": "/vault",
		"telegram_token": "tok",
		"telegram_chat_id": 1,
		"git_sync": {"enabled": false}
	}`))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitSync == nil {
		t.Fatal("git_sync should not be nil")
	}
	if cfg.GitSync.Enabled {
		t.Error("git_sync should be disabled")
	}
}

func TestGitSyncOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{
		"vault_path": "/vault",
		"telegram_token": "tok",
		"telegram_chat_id": 1
	}`))

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitSync != nil {
		t.Error("git_sync should be nil when omitted")
	}
}
