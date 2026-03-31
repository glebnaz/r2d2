package digest

import (
	"context"
	"strings"
	"testing"
	"time"

	"r2d2/internal/obsidian"
)

var loc = time.UTC

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func TestTasksCollector_SortsByPriority(t *testing.T) {
	now := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Low task", Due: now, Priority: "low"},
		{Title: "High task", Due: now, Priority: "high"},
	}
	c := NewTasksCollector(func() ([]obsidian.Task, error) { return tasks, nil }, loc)

	section, err := c.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	highIdx := strings.Index(section.Body, "High task")
	lowIdx := strings.Index(section.Body, "Low task")
	if highIdx > lowIdx {
		t.Error("high priority should come first")
	}
}

func TestTasksCollector_OverdueTasks(t *testing.T) {
	now := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Overdue", Due: date(2026, 3, 28)},
		{Title: "Today", Due: now},
	}
	c := NewTasksCollector(func() ([]obsidian.Task, error) { return tasks, nil }, loc)

	section, err := c.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section.Body, "Просрочено") {
		t.Error("expected overdue section")
	}
	if !strings.Contains(section.Body, "просрочено") {
		t.Error("expected overdue label")
	}
}

func TestTasksCollector_SkipsTimedTasks(t *testing.T) {
	now := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Timed", Due: time.Date(2026, 3, 30, 14, 0, 0, 0, loc), HasTime: true},
	}
	c := NewTasksCollector(func() ([]obsidian.Task, error) { return tasks, nil }, loc)

	section, err := c.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if section != nil {
		t.Error("expected nil section for timed-only tasks")
	}
}

func TestTasksCollector_Empty(t *testing.T) {
	c := NewTasksCollector(func() ([]obsidian.Task, error) { return nil, nil }, loc)

	section, err := c.Collect(context.Background(), date(2026, 3, 30))
	if err != nil {
		t.Fatal(err)
	}
	if section != nil {
		t.Error("expected nil for no tasks")
	}
}

func TestTasksCollector_WithDescription(t *testing.T) {
	now := date(2026, 3, 30)
	tasks := []obsidian.Task{
		{Title: "Task", Due: now, Description: "Some details"},
	}
	c := NewTasksCollector(func() ([]obsidian.Task, error) { return tasks, nil }, loc)

	section, err := c.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section.Body, "Some details") {
		t.Error("expected description")
	}
}

func TestTasksCollector_Header(t *testing.T) {
	now := date(2026, 3, 30)
	tasks := []obsidian.Task{{Title: "T", Due: now}}
	c := NewTasksCollector(func() ([]obsidian.Task, error) { return tasks, nil }, loc)

	section, err := c.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if section.Header != "📝 *Задачи*" {
		t.Errorf("unexpected header: %s", section.Header)
	}
}
