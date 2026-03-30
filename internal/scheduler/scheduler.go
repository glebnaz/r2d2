package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	formatDigest FormatFunc
	formatTimed  FormatFunc
	loc          *time.Location
	morningHour  int
	scanInterval time.Duration
	logger       *slog.Logger
	now          nowFunc

	mu       sync.Mutex
	sent     map[string]bool // tracks sent reminders: "filepath:YYYY-MM-DD" or "filepath:YYYY-MM-DDTHH:mm"
	timers   []*time.Timer
	stopCh   chan struct{}
	stopped  bool
}

// New creates a new Scheduler.
func New(
	scanFn ScanFunc,
	sender Sender,
	formatDigest FormatFunc,
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
		formatDigest: formatDigest,
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
	tasks, err := s.scanFn()
	if err != nil {
		s.logger.Error("failed to scan vault", "error", err)
		return
	}

	now := s.now().In(s.loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)

	s.mu.Lock()
	// Cancel existing timers before rebuilding.
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil
	s.mu.Unlock()

	var morningTasks []obsidian.Task
	morningTime := time.Date(now.Year(), now.Month(), now.Day(), s.morningHour, 0, 0, 0, s.loc)

	for _, task := range tasks {
		taskDate := time.Date(task.Due.Year(), task.Due.Month(), task.Due.Day(), 0, 0, 0, 0, s.loc)

		if task.HasTime {
			// Timed task: schedule at its specific time if it's today and not yet passed.
			if taskDate.Equal(today) {
				s.scheduleTimed(ctx, task, now)
			}
		} else {
			// Date-only task: include in morning digest if due today or overdue.
			if taskDate.Equal(today) || taskDate.Before(today) {
				morningTasks = append(morningTasks, task)
			}
		}
	}

	if len(morningTasks) > 0 {
		s.scheduleMorningDigest(ctx, morningTasks, morningTime, now)
	}

	s.logger.Info("schedule built", "morning_tasks", len(morningTasks), "total_scanned", len(tasks))
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
		// Time has passed, skip.
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
			s.logger.Error("failed to send timed reminder", "task", task.Title, "error", err)
			s.mu.Lock()
			delete(s.sent, key)
			s.mu.Unlock()
		} else {
			s.logger.Info("sent timed reminder", "task", task.Title)
		}
	})

	s.mu.Lock()
	s.timers = append(s.timers, timer)
	s.mu.Unlock()
}

// scheduleMorningDigest schedules a morning digest for date-only (and overdue) tasks.
func (s *Scheduler) scheduleMorningDigest(ctx context.Context, tasks []obsidian.Task, morningTime, now time.Time) {
	// Build a composite key for the digest.
	digestKey := fmt.Sprintf("digest:%s", now.Format("2006-01-02"))
	s.mu.Lock()
	if s.sent[digestKey] {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	delay := morningTime.Sub(now)
	if delay <= 0 {
		// Morning time has passed; send immediately.
		s.mu.Lock()
		if s.sent[digestKey] {
			s.mu.Unlock()
			return
		}
		s.sent[digestKey] = true
		s.mu.Unlock()

		msg := s.formatDigest(tasks)
		if err := s.sender.SendMessage(ctx, msg); err != nil {
			s.logger.Error("failed to send morning digest", "error", err)
			s.mu.Lock()
			delete(s.sent, digestKey)
			s.mu.Unlock()
		} else {
			s.logger.Info("sent morning digest", "tasks", len(tasks))
		}
		return
	}

	timer := time.AfterFunc(delay, func() {
		s.mu.Lock()
		if s.sent[digestKey] {
			s.mu.Unlock()
			return
		}
		s.sent[digestKey] = true
		s.mu.Unlock()

		// Re-scan to get the latest tasks for the digest.
		freshTasks, err := s.scanFn()
		if err != nil {
			s.logger.Error("failed to rescan for morning digest", "error", err)
			msg := s.formatDigest(tasks)
			if sendErr := s.sender.SendMessage(ctx, msg); sendErr != nil {
				s.logger.Error("failed to send morning digest", "error", sendErr)
				s.mu.Lock()
				delete(s.sent, digestKey)
				s.mu.Unlock()
			}
			return
		}

		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
		var digestTasks []obsidian.Task
		for _, t := range freshTasks {
			if t.HasTime {
				continue
			}
			taskDate := time.Date(t.Due.Year(), t.Due.Month(), t.Due.Day(), 0, 0, 0, 0, s.loc)
			if taskDate.Equal(today) || taskDate.Before(today) {
				digestTasks = append(digestTasks, t)
			}
		}

		if len(digestTasks) == 0 {
			digestTasks = tasks
		}

		msg := s.formatDigest(digestTasks)
		if sendErr := s.sender.SendMessage(ctx, msg); sendErr != nil {
			s.logger.Error("failed to send morning digest", "error", sendErr)
			s.mu.Lock()
			delete(s.sent, digestKey)
			s.mu.Unlock()
		} else {
			s.logger.Info("sent morning digest", "tasks", len(digestTasks))
		}
	})

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
