package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type GitSyncConfig struct {
	Enabled          bool   `json:"enabled"`
	RepoURL          string `json:"repo_url"`
	Branch           string `json:"branch"`
	WorkDir          string `json:"work_dir"`
	PushIntervalMin  int    `json:"push_interval_min"`
	AuthorName       string `json:"author_name"`
	AuthorEmail      string `json:"author_email"`
	NotifyOnPush     *bool  `json:"notify_on_push,omitempty"`
	NotifyOnConflict *bool  `json:"notify_on_conflict,omitempty"`
}

type Config struct {
	VaultPath           string         `json:"vault_path"`
	TelegramToken       string         `json:"telegram_token"`
	TelegramChatID      int64          `json:"telegram_chat_id"`
	Timezone            string         `json:"timezone"`
	MorningHour         int            `json:"morning_hour"`
	ScanIntervalMinutes int            `json:"scan_interval_minutes"`
	ReminderStatuses    []string       `json:"reminder_statuses"`
	GitSync             *GitSyncConfig `json:"git_sync,omitempty"`
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "r2d2", "config.json")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := &Config{
		Timezone:           "Europe/Moscow",
		MorningHour:        9,
		ScanIntervalMinutes: 5,
		ReminderStatuses:   []string{"todo", "in-progress", "block"},
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.VaultPath == "" {
		return nil, fmt.Errorf("vault_path is required")
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("telegram_token is required")
	}
	if cfg.TelegramChatID == 0 {
		return nil, fmt.Errorf("telegram_chat_id is required")
	}

	if cfg.GitSync != nil && cfg.GitSync.Enabled {
		if cfg.GitSync.RepoURL == "" {
			return nil, fmt.Errorf("git_sync.repo_url is required when git sync is enabled")
		}
		if cfg.GitSync.WorkDir == "" {
			return nil, fmt.Errorf("git_sync.work_dir is required when git sync is enabled")
		}
		if cfg.GitSync.Branch == "" {
			cfg.GitSync.Branch = "main"
		}
		if cfg.GitSync.PushIntervalMin == 0 {
			cfg.GitSync.PushIntervalMin = 30
		}
		if cfg.GitSync.AuthorName == "" {
			cfg.GitSync.AuthorName = "R2D2 Bot"
		}
		if cfg.GitSync.AuthorEmail == "" {
			cfg.GitSync.AuthorEmail = "r2d2@bot.local"
		}
		if cfg.GitSync.NotifyOnPush == nil {
			t := true
			cfg.GitSync.NotifyOnPush = &t
		}
		if cfg.GitSync.NotifyOnConflict == nil {
			t := true
			cfg.GitSync.NotifyOnConflict = &t
		}
	}

	return cfg, nil
}
