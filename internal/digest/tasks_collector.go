package digest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"r2d2/internal/obsidian"
)

var priorityOrder = map[string]int{
	"high": 0, "medium": 1, "low": 2, "": 3,
}

var priorityEmoji = map[string]string{
	"high": "🔴", "low": "🟢",
}

func priorityRank(p string) int {
	if r, ok := priorityOrder[p]; ok {
		return r
	}
	return 3
}

// ScanFunc scans the vault and returns tasks.
type ScanFunc func() ([]obsidian.Task, error)

// TasksCollector collects today's and overdue tasks for the morning digest.
type TasksCollector struct {
	scanFn ScanFunc
	loc    *time.Location
}

// NewTasksCollector creates a new tasks collector.
func NewTasksCollector(scanFn ScanFunc, loc *time.Location) *TasksCollector {
	return &TasksCollector{scanFn: scanFn, loc: loc}
}

// Collect returns a digest section with today's and overdue tasks.
func (c *TasksCollector) Collect(_ context.Context, now time.Time) (*Section, error) {
	tasks, err := c.scanFn()
	if err != nil {
		return nil, fmt.Errorf("scanning vault: %w", err)
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.loc)

	var dateOnly []obsidian.Task
	for _, t := range tasks {
		if t.HasTime {
			continue
		}
		taskDate := time.Date(t.Due.Year(), t.Due.Month(), t.Due.Day(), 0, 0, 0, 0, c.loc)
		if taskDate.Equal(today) || taskDate.Before(today) {
			dateOnly = append(dateOnly, t)
		}
	}

	if len(dateOnly) == 0 {
		return nil, nil
	}

	sort.Slice(dateOnly, func(i, j int) bool {
		ri, rj := priorityRank(dateOnly[i].Priority), priorityRank(dateOnly[j].Priority)
		if ri != rj {
			return ri < rj
		}
		return dateOnly[i].Title < dateOnly[j].Title
	})

	var overdue, due []obsidian.Task
	for _, t := range dateOnly {
		taskDate := time.Date(t.Due.Year(), t.Due.Month(), t.Due.Day(), 0, 0, 0, 0, c.loc)
		if taskDate.Before(today) {
			overdue = append(overdue, t)
		} else {
			due = append(due, t)
		}
	}

	var b strings.Builder
	if len(overdue) > 0 {
		b.WriteString("\n🚨 *Просрочено:*\n")
		for _, t := range overdue {
			b.WriteString(formatTaskCard(t, true))
		}
	}
	if len(due) > 0 {
		b.WriteString("\n📋 *Сегодня:*\n")
		for _, t := range due {
			b.WriteString(formatTaskCard(t, false))
		}
	}

	return &Section{
		Header: "📝 *Задачи*",
		Body:   b.String(),
	}, nil
}

func formatTaskCard(t obsidian.Task, overdue bool) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n📌 *%s*\n", escapeMarkdown(t.Title)))

	if emoji, ok := priorityEmoji[t.Priority]; ok {
		b.WriteString(fmt.Sprintf("  %s %s\n", emoji, t.Priority))
	}
	if t.Project != "" {
		b.WriteString(fmt.Sprintf("  📁 %s\n", escapeMarkdown(t.Project)))
	}
	if overdue {
		b.WriteString(fmt.Sprintf("  📅 %s _(просрочено)_\n", t.Due.Format("02.01.2006")))
	} else {
		b.WriteString(fmt.Sprintf("  📅 %s\n", t.Due.Format("02.01.2006")))
	}
	if t.Description != "" {
		b.WriteString(fmt.Sprintf("  💬 %s\n", escapeMarkdown(t.Description)))
	}

	return b.String()
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "`", "\\`",
	)
	return replacer.Replace(s)
}
