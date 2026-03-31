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

var priorityEmoji = map[string]string{
	"high": "🔴",
	"low":  "🟢",
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
	b.WriteString("☀️ *Задачи на сегодня*\n")
	b.WriteString(fmt.Sprintf("_%s_\n", today.Format("02 January 2006")))

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

	return b.String()
}

// FormatTimed formats a single timed task reminder message.
func FormatTimed(tasks []obsidian.Task) string {
	if len(tasks) == 0 {
		return ""
	}
	t := tasks[0]

	var b strings.Builder
	b.WriteString(fmt.Sprintf("⏰ *%s*\n", escapeMarkdown(t.Title)))

	if emoji, ok := priorityEmoji[t.Priority]; ok {
		b.WriteString(fmt.Sprintf("%s %s\n", emoji, t.Priority))
	}
	if t.Project != "" {
		b.WriteString(fmt.Sprintf("📁 %s\n", escapeMarkdown(t.Project)))
	}
	b.WriteString(fmt.Sprintf("🕐 %s", t.Due.Format("15:04")))

	if t.Description != "" {
		b.WriteString(fmt.Sprintf("\n\n%s", escapeMarkdown(t.Description)))
	}

	return b.String()
}

// MakeDigestFunc returns a FormatFunc that captures today's date for the digest.
func MakeDigestFunc(now func() time.Time) func([]obsidian.Task) string {
	return func(tasks []obsidian.Task) string {
		return FormatDigest(now(), tasks)
	}
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
