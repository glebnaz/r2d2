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
	NotifyOnPush     bool   `json:"notify_on_push"`
	NotifyOnConflict bool   `json:"notify_on_conflict"`
}

// gitSyncRaw is used for JSON unmarshaling to distinguish "not set" from "set to false".
type gitSyncRaw struct {
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

// configRaw is used for JSON unmarshaling with *bool fields.
type configRaw struct {
	VaultPath           string       `json:"vault_path"`
	TelegramToken       string       `json:"telegram_token"`
	TelegramChatID      int64        `json:"telegram_chat_id"`
	Timezone            string       `json:"timezone"`
	MorningHour         int          `json:"morning_hour"`
	ScanIntervalMinutes int          `json:"scan_interval_minutes"`
	ReminderStatuses    []string     `json:"reminder_statuses"`
	GitSync             *gitSyncRaw  `json:"git_sync,omitempty"`
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

	raw := &configRaw{
		Timezone:           "Europe/Moscow",
		MorningHour:        9,
		ScanIntervalMinutes: 5,
		ReminderStatuses:   []string{"todo", "in-progress", "block"},
	}

	if err := json.Unmarshal(data, raw); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg := &Config{
		VaultPath:           raw.VaultPath,
		TelegramToken:       raw.TelegramToken,
		TelegramChatID:      raw.TelegramChatID,
		Timezone:            raw.Timezone,
		MorningHour:         raw.MorningHour,
		ScanIntervalMinutes: raw.ScanIntervalMinutes,
		ReminderStatuses:    raw.ReminderStatuses,
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

	if raw.GitSync != nil {
		gs := &GitSyncConfig{
			Enabled:     raw.GitSync.Enabled,
			RepoURL:     raw.GitSync.RepoURL,
			Branch:      raw.GitSync.Branch,
			WorkDir:     raw.GitSync.WorkDir,
			PushIntervalMin: raw.GitSync.PushIntervalMin,
			AuthorName:  raw.GitSync.AuthorName,
			AuthorEmail: raw.GitSync.AuthorEmail,
		}

		if gs.Enabled {
			if gs.RepoURL == "" {
				return nil, fmt.Errorf("git_sync.repo_url is required when git sync is enabled")
			}
			if gs.WorkDir == "" {
				return nil, fmt.Errorf("git_sync.work_dir is required when git sync is enabled")
			}
			if gs.Branch == "" {
				gs.Branch = "main"
			}
			if gs.PushIntervalMin < 1 {
				gs.PushIntervalMin = 30
			}
			if gs.AuthorName == "" {
				gs.AuthorName = "R2D2 Bot"
			}
			if gs.AuthorEmail == "" {
				gs.AuthorEmail = "r2d2@bot.local"
			}
		}

		// Default notify flags to true if not explicitly set.
		if raw.GitSync.NotifyOnPush == nil {
			gs.NotifyOnPush = true
		} else {
			gs.NotifyOnPush = *raw.GitSync.NotifyOnPush
		}
		if raw.GitSync.NotifyOnConflict == nil {
			gs.NotifyOnConflict = true
		} else {
			gs.NotifyOnConflict = *raw.GitSync.NotifyOnConflict
		}

		cfg.GitSync = gs
	}

	return cfg, nil
}
