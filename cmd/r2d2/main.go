package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"r2d2/internal/config"
	"r2d2/internal/digest"
	"r2d2/internal/metrics"
	"r2d2/internal/obsidian"
	"r2d2/internal/reminder"
	"r2d2/internal/scheduler"
	"r2d2/internal/telegram"
)

// dryRunSender prints messages to stdout instead of sending via Telegram.
type dryRunSender struct{}

func (d *dryRunSender) SendMessage(_ context.Context, text string) error {
	fmt.Println("--- DRY RUN ---")
	fmt.Println(text)
	fmt.Println("--- END ---")
	return nil
}

func run() error {
	configPath := flag.String("config", "", "path to config file")
	dryRun := flag.Bool("dry-run", false, "print reminders to stdout instead of sending via Telegram")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("loading timezone %q: %w", cfg.Timezone, err)
	}

	logger.Info("r2d2 starting",
		"vault", cfg.VaultPath,
		"timezone", cfg.Timezone,
		"morning_hour", cfg.MorningHour,
		"scan_interval", cfg.ScanIntervalMinutes,
		"dry_run", *dryRun,
	)

	scanFn := func() ([]obsidian.Task, error) {
		return obsidian.ScanVault(cfg.VaultPath, cfg.ReminderStatuses, loc)
	}

	var sender scheduler.Sender
	if *dryRun {
		sender = &dryRunSender{}
	} else {
		tgClient, err := telegram.New(cfg.TelegramToken, cfg.TelegramChatID)
		if err != nil {
			return fmt.Errorf("creating telegram client: %w", err)
		}
		sender = tgClient
	}

	// Build digest engine with collectors.
	engine := digest.NewEngine()
	engine.Register(digest.NewTasksCollector(scanFn, loc))

	sched := scheduler.New(
		scanFn,
		sender,
		engine,
		reminder.FormatTimed,
		loc,
		cfg.MorningHour,
		cfg.ScanIntervalMinutes,
		logger,
	)

	// Start metrics HTTP server.
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	metricsServer := &http.Server{Addr: ":9182", Handler: mux}
	go func() {
		logger.Info("metrics server starting", "addr", ":9182")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	if err := sched.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("scheduler error: %w", err)
	}

	_ = metricsServer.Close()
	logger.Info("r2d2 stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
