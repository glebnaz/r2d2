package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"r2d2/internal/digest"
	"r2d2/internal/metrics"
	"r2d2/internal/obsidian"
)

// Sender is the interface for sending reminder messages.
type Sender interface {
	SendMessage(ctx context.Context, text string) error
}

// FormatFunc formats a list of tasks into a reminder message string.
type FormatFunc func(tasks []obsidian.Task) string

// ScanFunc scans the vault and returns tasks.
type ScanFunc func() ([]obsidian.Task, error)

// nowFunc allows overriding time.Now for testing.
type nowFunc func() time.Time

// Scheduler manages reminder scheduling for Obsidian tasks.
type Scheduler struct {
	scanFn       ScanFunc
	sender       Sender
	digestEngine *digest.Engine
	formatTimed  FormatFunc
	loc          *time.Location
	morningHour  int
	scanInterval time.Duration
	logger       *slog.Logger
	now          nowFunc

	mu      sync.Mutex
	sent    map[string]bool // tracks sent reminders: "filepath:YYYY-MM-DD" or "filepath:YYYY-MM-DDTHH:mm"
	timers  []*time.Timer
	stopCh  chan struct{}
	stopped bool
}

// New creates a new Scheduler.
func New(
	scanFn ScanFunc,
	sender Sender,
	digestEngine *digest.Engine,
	formatTimed FormatFunc,
	loc *time.Location,
	morningHour int,
	scanIntervalMinutes int,
	logger *slog.Logger,
) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		scanFn:       scanFn,
		sender:       sender,
		digestEngine: digestEngine,
		formatTimed:  formatTimed,
		loc:          loc,
		morningHour:  morningHour,
		scanInterval: time.Duration(scanIntervalMinutes) * time.Minute,
		logger:       logger,
		now:          time.Now,
		sent:         make(map[string]bool),
		stopCh:       make(chan struct{}),
	}
}

// sentKey returns a dedup key for a task reminder.
func sentKey(task obsidian.Task) string {
	if task.HasTime {
		return fmt.Sprintf("%s:%s", task.FilePath, task.Due.Format("2006-01-02T15:04"))
	}
	return fmt.Sprintf("%s:%s", task.FilePath, task.Due.Format("2006-01-02"))
}

// Run starts the scheduler. It blocks until the context is cancelled or Stop is called.
func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info("scheduler starting")

	s.buildSchedule(ctx)

	scanTicker := time.NewTicker(s.scanInterval)
	defer scanTicker.Stop()

	midnightTimer := s.newMidnightTimer()
	defer midnightTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Stop()
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-scanTicker.C:
			s.logger.Info("periodic rescan triggered")
			s.buildSchedule(ctx)
		case <-midnightTimer.C:
			s.logger.Info("midnight: rebuilding schedule for new day")
			s.mu.Lock()
			s.sent = make(map[string]bool)
			s.mu.Unlock()
			s.buildSchedule(ctx)
			midnightTimer.Stop()
			midnightTimer = s.newMidnightTimer()
		}
	}
}

// Stop stops the scheduler and cancels all pending timers.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.stopCh)
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil
	s.logger.Info("scheduler stopped")
}

// newMidnightTimer returns a timer that fires at the next midnight in the configured timezone.
func (s *Scheduler) newMidnightTimer() *time.Timer {
	now := s.now().In(s.loc)
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, s.loc)
	return time.NewTimer(nextMidnight.Sub(now))
}

// buildSchedule scans the vault and schedules reminders for today's tasks.
func (s *Scheduler) buildSchedule(ctx context.Context) {
	scanStart := time.Now()
	tasks, err := s.scanFn()
	metrics.VaultScanDuration.Observe(time.Since(scanStart).Seconds())
	metrics.VaultScans.Inc()

	if err != nil {
		metrics.VaultScanErrors.Inc()
		s.logger.Error("failed to scan vault", "error", err)
		return
	}

	metrics.TasksScanned.Set(float64(len(tasks)))

	now := s.now().In(s.loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)

	s.mu.Lock()
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil
	s.mu.Unlock()

	morningTime := time.Date(now.Year(), now.Month(), now.Day(), s.morningHour, 0, 0, 0, s.loc)
	hasMorningWork := false
	var dueToday, overdueCount int

	for _, task := range tasks {
		taskDate := time.Date(task.Due.Year(), task.Due.Month(), task.Due.Day(), 0, 0, 0, 0, s.loc)

		if task.HasTime {
			if taskDate.Equal(today) {
				s.scheduleTimed(ctx, task, now)
				dueToday++
			}
		} else {
			if taskDate.Before(today) {
				hasMorningWork = true
				overdueCount++
			} else if taskDate.Equal(today) {
				hasMorningWork = true
				dueToday++
			}
		}
	}

	metrics.TasksDueToday.Set(float64(dueToday))
	metrics.TasksOverdue.Set(float64(overdueCount))

	if hasMorningWork {
		s.scheduleMorningDigest(ctx, morningTime, now)
	}

	s.logger.Info("schedule built", "total_scanned", len(tasks))
}

// scheduleTimed schedules a single timed task reminder.
func (s *Scheduler) scheduleTimed(ctx context.Context, task obsidian.Task, now time.Time) {
	key := sentKey(task)
	s.mu.Lock()
	if s.sent[key] {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	delay := task.Due.Sub(now)
	if delay <= 0 {
		return
	}

	timer := time.AfterFunc(delay, func() {
		s.mu.Lock()
		if s.sent[key] {
			s.mu.Unlock()
			return
		}
		s.sent[key] = true
		s.mu.Unlock()

		msg := s.formatTimed([]obsidian.Task{task})
		if err := s.sender.SendMessage(ctx, msg); err != nil {
			metrics.NotificationErrors.Inc()
			s.logger.Error("failed to send timed reminder", "task", task.Title, "error", err)
			s.mu.Lock()
			delete(s.sent, key)
			s.mu.Unlock()
		} else {
			metrics.TimedRemindersSent.Inc()
			s.logger.Info("sent timed reminder", "task", task.Title)
		}
	})

	s.mu.Lock()
	s.timers = append(s.timers, timer)
	s.mu.Unlock()
}

// scheduleMorningDigest schedules the morning digest using the digest engine.
func (s *Scheduler) scheduleMorningDigest(ctx context.Context, morningTime, now time.Time) {
	digestKey := fmt.Sprintf("digest:%s", now.Format("2006-01-02"))
	s.mu.Lock()
	if s.sent[digestKey] {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	sendDigest := func() {
		s.mu.Lock()
		if s.sent[digestKey] {
			s.mu.Unlock()
			return
		}
		s.sent[digestKey] = true
		s.mu.Unlock()

		msg, err := s.digestEngine.Build(ctx, s.now().In(s.loc))
		if err != nil {
			s.logger.Error("failed to build digest", "error", err)
			s.mu.Lock()
			delete(s.sent, digestKey)
			s.mu.Unlock()
			return
		}
		if msg == "" {
			return
		}
		if err := s.sender.SendMessage(ctx, msg); err != nil {
			metrics.NotificationErrors.Inc()
			s.logger.Error("failed to send morning digest", "error", err)
			s.mu.Lock()
			delete(s.sent, digestKey)
			s.mu.Unlock()
		} else {
			metrics.DigestsSent.Inc()
			s.logger.Info("sent morning digest")
		}
	}

	delay := morningTime.Sub(now)
	if delay <= 0 {
		sendDigest()
		return
	}

	timer := time.AfterFunc(delay, sendDigest)
	s.mu.Lock()
	s.timers = append(s.timers, timer)
	s.mu.Unlock()
}

// SentCount returns the number of sent reminders (for testing).
func (s *Scheduler) SentCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

// IsSent checks if a reminder has been sent for the given key (for testing).
func (s *Scheduler) IsSent(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent[key]
}

// PendingTimers returns the number of pending timers (for testing).
func (s *Scheduler) PendingTimers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.timers)
}
