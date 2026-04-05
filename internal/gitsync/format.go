package gitsync

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// FormatPushNotification formats a successful push notification in Russian.
// fileNames is a newline-separated list of changed file paths from git diff --name-only.
func FormatPushNotification(filesChanged int, fileNames string, timestamp time.Time) string {
	var b strings.Builder
	b.WriteString("📤 *Git Sync*\n")
	fmt.Fprintf(&b, "_%s_\n\n", timestamp.Format("02.01.2006 15:04"))

	if fileNames = strings.TrimSpace(fileNames); fileNames != "" {
		lines := strings.Split(fileNames, "\n")
		const maxLines = 10
		for i, line := range lines {
			if i >= maxLines {
				fmt.Fprintf(&b, "  _…и ещё %d_\n", len(lines)-maxLines)
				break
			}
			// Show just the filename without directory for cleaner look.
			name := filepath.Base(strings.TrimSpace(line))
			b.WriteString("  • " + name + "\n")
		}
	} else {
		fmt.Fprintf(&b, "Файлов: %d\n", filesChanged)
	}

	return b.String()
}

// FormatConflictAlert formats an urgent push conflict notification in Russian.
func FormatConflictAlert(stderr string, timestamp time.Time) string {
	var b strings.Builder
	b.WriteString("🚨 *Git Sync — конфликт!*\n")
	fmt.Fprintf(&b, "_%s_\n\n", timestamp.Format("02.01.2006 15:04"))
	b.WriteString("Push отклонён. Требуется ручное вмешательство.\n")
	return b.String()
}
