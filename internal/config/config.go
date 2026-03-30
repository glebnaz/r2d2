package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	VaultPath          string   `json:"vault_path"`
	TelegramToken      string   `json:"telegram_token"`
	TelegramChatID     int64    `json:"telegram_chat_id"`
	Timezone           string   `json:"timezone"`
	MorningHour        int      `json:"morning_hour"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	ReminderStatuses   []string `json:"reminder_statuses"`
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

	return cfg, nil
}
