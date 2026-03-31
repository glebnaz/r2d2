package reminder

import (
	"strings"
	"testing"
	"time"

	"r2d2/internal/obsidian"
)

var loc = time.UTC

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func datetime(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, loc)
}

func TestFormatDigest_SortsByPriority(t *testing.T) {
	today := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Low task", Due: today, Priority: "low"},
		{Title: "High task", Due: today, Priority: "high"},
		{Title: "Medium task", Due: today, Priority: "medium"},
	}

	result := FormatDigest(today, tasks)

	highIdx := strings.Index(result, "High task")
	medIdx := strings.Index(result, "Medium task")
	lowIdx := strings.Index(result, "Low task")

	if highIdx > medIdx || medIdx > lowIdx {
		t.Errorf("tasks not sorted by priority:\n%s", result)
	}
}

func TestFormatDigest_OverdueTasks(t *testing.T) {
	today := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Overdue task", Due: date(2026, 3, 28), Priority: "high"},
		{Title: "Today task", Due: today, Priority: "medium"},
	}

	result := FormatDigest(today, tasks)

	if !strings.Contains(result, "Просрочено:") {
		t.Error("expected overdue section")
	}
	if !strings.Contains(result, "Overdue task") {
		t.Error("expected overdue task in output")
	}
	if !strings.Contains(result, "Сегодня:") {
		t.Error("expected due today section")
	}
	if !strings.Contains(result, "просрочено") {
		t.Errorf("expected overdue label, got:\n%s", result)
	}
}

func TestFormatDigest_IncludesProjectAndPriority(t *testing.T) {
	today := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "My task", Due: today, Priority: "high", Project: "R2D2"},
	}

	result := FormatDigest(today, tasks)

	if !strings.Contains(result, "🔴") {
		t.Errorf("expected priority emoji in output:\n%s", result)
	}
	if !strings.Contains(result, "R2D2") {
		t.Errorf("expected project in output:\n%s", result)
	}
}

func TestFormatDigest_Empty(t *testing.T) {
	result := FormatDigest(date(2026, 3, 30), nil)
	if result != "" {
		t.Errorf("expected empty string for no tasks, got: %s", result)
	}
}

func TestFormatDigest_NoPriority(t *testing.T) {
	today := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Simple task", Due: today},
	}

	result := FormatDigest(today, tasks)

	if strings.Contains(result, "🔴") || strings.Contains(result, "🟢") {
		t.Error("should not show priority emoji when no priority")
	}
}

func TestFormatDigest_WithDescription(t *testing.T) {
	today := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Task", Due: today, Description: "Some details here"},
	}

	result := FormatDigest(today, tasks)

	if !strings.Contains(result, "Some details here") {
		t.Errorf("expected description in output:\n%s", result)
	}
}

func TestFormatTimed_SingleTask(t *testing.T) {
	tasks := []obsidian.Task{
		{Title: "Meeting", Due: datetime(2026, 3, 30, 14, 30), HasTime: true, Priority: "high", Project: "Work"},
	}

	result := FormatTimed(tasks)

	if !strings.Contains(result, "Meeting") {
		t.Error("expected task title")
	}
	if !strings.Contains(result, "14:30") {
		t.Error("expected time")
	}
	if !strings.Contains(result, "🔴") {
		t.Error("expected priority emoji")
	}
	if !strings.Contains(result, "Work") {
		t.Error("expected project")
	}
}

func TestFormatTimed_WithDescription(t *testing.T) {
	tasks := []obsidian.Task{
		{Title: "Call", Due: datetime(2026, 3, 30, 14, 30), HasTime: true, Description: "Call John about the project"},
	}

	result := FormatTimed(tasks)

	if !strings.Contains(result, "Call John about the project") {
		t.Errorf("expected description in output:\n%s", result)
	}
}

func TestFormatTimed_MinimalTask(t *testing.T) {
	tasks := []obsidian.Task{
		{Title: "Quick call", Due: datetime(2026, 3, 30, 10, 0), HasTime: true},
	}

	result := FormatTimed(tasks)

	if !strings.Contains(result, "Quick call") {
		t.Error("expected task title")
	}
	if strings.Contains(result, "🔴") || strings.Contains(result, "🟢") {
		t.Error("should not show priority when empty")
	}
	if strings.Contains(result, "📁") {
		t.Error("should not show project when empty")
	}
}

func TestFormatTimed_Empty(t *testing.T) {
	result := FormatTimed(nil)
	if result != "" {
		t.Errorf("expected empty string for no tasks, got: %s", result)
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"_italic_", "\\_italic\\_"},
		{"*bold*", "\\*bold\\*"},
		{"[link]", "\\[link\\]"},
		{"`code`", "\\`code\\`"},
	}

	for _, tt := range tests {
		got := escapeMarkdown(tt.input)
		if got != tt.expected {
			t.Errorf("escapeMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMakeDigestFunc(t *testing.T) {
	now := func() time.Time { return date(2026, 3, 30) }
	fn := MakeDigestFunc(now)

	tasks := []obsidian.Task{
		{Title: "Test task", Due: date(2026, 3, 30)},
	}

	result := fn(tasks)
	if !strings.Contains(result, "Test task") {
		t.Error("MakeDigestFunc should produce valid digest")
	}
}

func TestFormatDigest_OverdueBeforeDueToday(t *testing.T) {
	today := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Today task", Due: today},
		{Title: "Overdue task", Due: date(2026, 3, 25)},
	}

	result := FormatDigest(today, tasks)

	overdueIdx := strings.Index(result, "Просрочено:")
	dueTodayIdx := strings.Index(result, "Сегодня:")

	if overdueIdx > dueTodayIdx {
		t.Error("overdue section should appear before due today section")
	}
}
