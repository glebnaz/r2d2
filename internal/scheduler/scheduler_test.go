package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"r2d2/internal/obsidian"
)

// mockSender records messages sent.
type mockSender struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockSender) SendMessage(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, text)
	return nil
}

func (m *mockSender) Messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.messages))
	copy(result, m.messages)
	return result
}

// failingSender always returns an error.
type failingSender struct{}

func (f *failingSender) SendMessage(_ context.Context, _ string) error {
	return fmt.Errorf("send failed")
}

func testLocation() *time.Location {
	loc, _ := time.LoadLocation("Europe/Moscow")
	return loc
}

func simpleDigestFormat(tasks []obsidian.Task) string {
	var titles []string
	for _, t := range tasks {
		titles = append(titles, t.Title)
	}
	return "digest:" + strings.Join(titles, ",")
}

func simpleTimedFormat(tasks []obsidian.Task) string {
	if len(tasks) == 0 {
		return ""
	}
	return "timed:" + tasks[0].Title
}

func TestScheduler_DateOnlyTask_MorningDigest_Immediate(t *testing.T) {
	loc := testLocation()
	// Set "now" to 10:00, morning hour is 9:00 -> digest should send immediately.
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Buy groceries",
			Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
			HasTime:  false,
			Priority: "high",
			FilePath: "/vault/buy-groceries.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60, // long interval so rescan doesn't fire
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx, cancel := context.WithCancel(context.Background())

	// Run buildSchedule directly (not Run, which blocks).
	s.buildSchedule(ctx)
	cancel()

	msgs := sender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0] != "digest:Buy groceries" {
		t.Errorf("unexpected message: %s", msgs[0])
	}
}

func TestScheduler_DateOnlyTask_MorningDigest_Scheduled(t *testing.T) {
	loc := testLocation()
	// Set "now" to 08:59:59, morning hour is 9:00 -> digest should be scheduled.
	fakeNow := time.Date(2026, 3, 30, 8, 59, 59, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Review PR",
			Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/review-pr.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	// Should have a pending timer, no messages yet.
	if s.PendingTimers() == 0 {
		t.Fatal("expected pending timer for morning digest")
	}
	msgs := sender.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}

	s.Stop()
}

func TestScheduler_TimedTask_Scheduled(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 14, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Team standup",
			Due:      time.Date(2026, 3, 30, 15, 0, 0, 0, loc),
			HasTime:  true,
			FilePath: "/vault/team-standup.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	if s.PendingTimers() != 1 {
		t.Fatalf("expected 1 pending timer, got %d", s.PendingTimers())
	}
	msgs := sender.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}

	s.Stop()
}

func TestScheduler_TimedTask_AlreadyPassed(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 16, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Morning meeting",
			Due:      time.Date(2026, 3, 30, 10, 0, 0, 0, loc),
			HasTime:  true,
			FilePath: "/vault/morning-meeting.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	// Timed task in the past should be skipped entirely.
	if s.PendingTimers() != 0 {
		t.Fatalf("expected 0 pending timers, got %d", s.PendingTimers())
	}
	msgs := sender.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

func TestScheduler_OverdueTask_IncludedInDigest(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Overdue report",
			Due:      time.Date(2026, 3, 28, 0, 0, 0, 0, loc), // 2 days ago
			HasTime:  false,
			FilePath: "/vault/overdue-report.md",
		},
		{
			Title:    "Today task",
			Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/today-task.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	msgs := sender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "Overdue report") || !strings.Contains(msgs[0], "Today task") {
		t.Errorf("digest should contain both overdue and today tasks: %s", msgs[0])
	}
}

func TestScheduler_DuplicatePrevention(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Daily standup",
			Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/daily-standup.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()

	// Build schedule twice - should only send digest once.
	s.buildSchedule(ctx)
	s.buildSchedule(ctx)

	msgs := sender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (dedup), got %d", len(msgs))
	}
}

func TestScheduler_FutureDateTask_NotScheduled(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Future task",
			Due:      time.Date(2026, 4, 5, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/future-task.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	msgs := sender.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for future task, got %d", len(msgs))
	}
}

func TestScheduler_ScanError_DoesNotCrash(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return nil, fmt.Errorf("vault not found") },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	// Should not panic.
	s.buildSchedule(ctx)

	msgs := sender.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages on scan error, got %d", len(msgs))
	}
}

func TestScheduler_SendError_ClearsSentFlag(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Failing task",
			Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/failing-task.md",
		},
	}

	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		&failingSender{},
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	// Digest key should be cleared after send failure, allowing retry.
	digestKey := fmt.Sprintf("digest:%s", fakeNow.Format("2006-01-02"))
	if s.IsSent(digestKey) {
		t.Error("digest key should be cleared after send failure")
	}
}

func TestScheduler_RunAndStop(t *testing.T) {
	loc := testLocation()
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return nil, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx)
	}()

	// Give it a moment to start.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop within timeout")
	}
}

func TestScheduler_StopIdempotent(t *testing.T) {
	loc := testLocation()
	s := New(
		func() ([]obsidian.Task, error) { return nil, nil },
		&mockSender{},
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		nil,
	)
	// Calling Stop multiple times should not panic.
	s.Stop()
	s.Stop()
}

func TestSentKey(t *testing.T) {
	loc := testLocation()

	timedTask := obsidian.Task{
		FilePath: "/vault/task.md",
		Due:      time.Date(2026, 3, 30, 14, 30, 0, 0, loc),
		HasTime:  true,
	}
	key := sentKey(timedTask)
	if key != "/vault/task.md:2026-03-30T14:30" {
		t.Errorf("unexpected timed key: %s", key)
	}

	dateTask := obsidian.Task{
		FilePath: "/vault/task.md",
		Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
		HasTime:  false,
	}
	key = sentKey(dateTask)
	if key != "/vault/task.md:2026-03-30" {
		t.Errorf("unexpected date key: %s", key)
	}
}

func TestScheduler_MixedTasks(t *testing.T) {
	loc := testLocation()
	// 10:00 - morning passed, timed task at 15:00 still ahead.
	fakeNow := time.Date(2026, 3, 30, 10, 0, 0, 0, loc)

	tasks := []obsidian.Task{
		{
			Title:    "Date task",
			Due:      time.Date(2026, 3, 30, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/date-task.md",
		},
		{
			Title:    "Timed task",
			Due:      time.Date(2026, 3, 30, 15, 0, 0, 0, loc),
			HasTime:  true,
			FilePath: "/vault/timed-task.md",
		},
		{
			Title:    "Tomorrow task",
			Due:      time.Date(2026, 3, 31, 0, 0, 0, 0, loc),
			HasTime:  false,
			FilePath: "/vault/tomorrow-task.md",
		},
	}

	sender := &mockSender{}
	s := New(
		func() ([]obsidian.Task, error) { return tasks, nil },
		sender,
		simpleDigestFormat,
		simpleTimedFormat,
		loc,
		9,
		60,
		slog.Default(),
	)
	s.now = func() time.Time { return fakeNow }

	ctx := context.Background()
	s.buildSchedule(ctx)

	// Should have: 1 digest sent immediately, 1 timer for timed task.
	msgs := sender.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 digest message, got %d", len(msgs))
	}
	if msgs[0] != "digest:Date task" {
		t.Errorf("unexpected digest: %s", msgs[0])
	}
	if s.PendingTimers() != 1 {
		t.Fatalf("expected 1 pending timer for timed task, got %d", s.PendingTimers())
	}

	s.Stop()
}
