package reminder

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"r2d2/internal/obsidian"
)

// priorityOrder defines sort order for priorities (lower = higher priority).
var priorityOrder = map[string]int{
	"high":   0,
	"medium": 1,
	"low":    2,
	"":       3,
}

func priorityRank(p string) int {
	if r, ok := priorityOrder[p]; ok {
		return r
	}
	return 3
}

// FormatDigest formats a morning digest message for date-only tasks.
// Tasks are sorted by priority (high first), then overdue tasks are labeled.
func FormatDigest(today time.Time, tasks []obsidian.Task) string {
	if len(tasks) == 0 {
		return ""
	}

	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	sorted := make([]obsidian.Task, len(tasks))
	copy(sorted, tasks)
	sort.Slice(sorted, func(i, j int) bool {
		ri, rj := priorityRank(sorted[i].Priority), priorityRank(sorted[j].Priority)
		if ri != rj {
			return ri < rj
		}
		return sorted[i].Title < sorted[j].Title
	})

	var overdue, due []obsidian.Task
	for _, t := range sorted {
		taskDate := time.Date(t.Due.Year(), t.Due.Month(), t.Due.Day(), 0, 0, 0, 0, t.Due.Location())
		if taskDate.Before(todayDate) {
			overdue = append(overdue, t)
		} else {
			due = append(due, t)
		}
	}

	var b strings.Builder
	b.WriteString("*Morning Task Digest*\n")

	if len(overdue) > 0 {
		b.WriteString("\n⚠️ *Overdue:*\n")
		for _, t := range overdue {
			b.WriteString(formatTaskLine(t, true))
		}
	}

	if len(due) > 0 {
		b.WriteString("\n📋 *Due Today:*\n")
		for _, t := range due {
			b.WriteString(formatTaskLine(t, false))
		}
	}

	return b.String()
}

// FormatTimed formats a single timed task reminder message.
func FormatTimed(tasks []obsidian.Task) string {
	if len(tasks) == 0 {
		return ""
	}
	t := tasks[0]

	var b strings.Builder
	b.WriteString(fmt.Sprintf("⏰ *Reminder:* %s\n", escapeMarkdown(t.Title)))

	if t.Priority != "" {
		b.WriteString(fmt.Sprintf("Priority: %s\n", t.Priority))
	}
	if t.Project != "" {
		b.WriteString(fmt.Sprintf("Project: %s\n", escapeMarkdown(t.Project)))
	}
	b.WriteString(fmt.Sprintf("Due: %s", t.Due.Format("15:04")))

	return b.String()
}

// MakeDigestFunc returns a FormatFunc that captures today's date for the digest.
func MakeDigestFunc(now func() time.Time) func([]obsidian.Task) string {
	return func(tasks []obsidian.Task) string {
		return FormatDigest(now(), tasks)
	}
}

func formatTaskLine(t obsidian.Task, overdue bool) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("• %s", escapeMarkdown(t.Title)))
	if t.Priority != "" {
		parts = append(parts, fmt.Sprintf("[%s]", t.Priority))
	}
	if t.Project != "" {
		parts = append(parts, fmt.Sprintf("_%s_", escapeMarkdown(t.Project)))
	}
	if overdue {
		parts = append(parts, fmt.Sprintf("(due %s)", t.Due.Format("Jan 02")))
	}
	return strings.Join(parts, " ") + "\n"
}

// escapeMarkdown escapes special Telegram Markdown v1 characters.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"`", "\\`",
	)
	return replacer.Replace(s)
}
