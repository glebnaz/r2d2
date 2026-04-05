package gitsync

import (
	"fmt"
	"strings"
	"time"
)

// FormatPushNotification formats a successful push notification in Russian.
func FormatPushNotification(filesChanged int, summary string, timestamp time.Time) string {
	var b strings.Builder
	b.WriteString("📤 *Git Sync*\n")
	fmt.Fprintf(&b, "_%s_\n", timestamp.Format("02.01.2006 15:04"))
	fmt.Fprintf(&b, "\nФайлов изменено: %d\n", filesChanged)
	if summary = strings.TrimSpace(summary); summary != "" {
		fmt.Fprintf(&b, "\n```\n%s\n```", summary)
	}
	return b.String()
}

// FormatConflictAlert formats an urgent push conflict notification in Russian.
func FormatConflictAlert(stderr string, timestamp time.Time) string {
	var b strings.Builder
	b.WriteString("🚨 *Git Sync — конфликт!*\n")
	fmt.Fprintf(&b, "_%s_\n", timestamp.Format("02.01.2006 15:04"))
	b.WriteString("\nНе удалось отправить изменения — удалённая ветка обновилась. Локальный коммит сброшен, повтор при следующей синхронизации.\n")
	if stderr = strings.TrimSpace(stderr); stderr != "" {
		fmt.Fprintf(&b, "\n```\n%s\n```", stderr)
	}
	return b.String()
}
