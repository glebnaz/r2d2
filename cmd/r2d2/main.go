package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"r2d2/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	dryRun := flag.Bool("dry-run", false, "print reminders to stdout instead of sending via Telegram")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	_ = dryRun

	fmt.Printf("r2d2 starting with vault: %s\n", cfg.VaultPath)
}
