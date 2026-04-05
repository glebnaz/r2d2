package gitsync

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 4, 5, 14, 30, 0, 0, time.UTC)

func TestFormatPushNotification_Full(t *testing.T) {
	summary := " README.md | 2 ++\n main.go   | 5 ++---"
	result := FormatPushNotification(2, summary, testTime)

	checks := []string{
		"📤",
		"Git Sync",
		"05.04.2026 14:30",
		"Файлов изменено: 2",
		"README.md",
		"main.go",
	}
	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output:\n%s", want, result)
		}
	}
}

func TestFormatPushNotification_EmptySummary(t *testing.T) {
	result := FormatPushNotification(1, "", testTime)

	if !strings.Contains(result, "Файлов изменено: 1") {
		t.Errorf("expected file count in output:\n%s", result)
	}
	if strings.Contains(result, "```") {
		t.Errorf("should not contain code block when summary is empty:\n%s", result)
	}
}

func TestFormatPushNotification_WhitespaceSummary(t *testing.T) {
	result := FormatPushNotification(3, "   \n  ", testTime)

	if strings.Contains(result, "```") {
		t.Errorf("should not contain code block when summary is only whitespace:\n%s", result)
	}
}

func TestFormatConflictAlert_Full(t *testing.T) {
	stderr := "error: failed to push some refs to 'origin'\nhint: non-fast-forward"
	result := FormatConflictAlert(stderr, testTime)

	checks := []string{
		"🚨",
		"конфликт",
		"05.04.2026 14:30",
		"Не удалось отправить изменения",
		"Локальный коммит сброшен",
		"non-fast-forward",
	}
	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output:\n%s", want, result)
		}
	}
}

func TestFormatConflictAlert_EmptyStderr(t *testing.T) {
	result := FormatConflictAlert("", testTime)

	if !strings.Contains(result, "конфликт") {
		t.Errorf("expected conflict header:\n%s", result)
	}
	if strings.Contains(result, "```") {
		t.Errorf("should not contain code block when stderr is empty:\n%s", result)
	}
}

func TestFormatConflictAlert_WhitespaceStderr(t *testing.T) {
	result := FormatConflictAlert("   \n  ", testTime)

	if strings.Contains(result, "```") {
		t.Errorf("should not contain code block when stderr is only whitespace:\n%s", result)
	}
}
