package gitsync

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 4, 5, 14, 30, 0, 0, time.UTC)

func TestFormatPushNotification_Full(t *testing.T) {
	fileNames := "README.md\nmain.go"
	result := FormatPushNotification(2, fileNames, testTime)

	checks := []string{
		"📤",
		"Git Sync",
		"05.04.2026 14:30",
		"README.md",
		"main.go",
		"•",
	}
	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output:\n%s", want, result)
		}
	}
}

func TestFormatPushNotification_EmptyNames(t *testing.T) {
	result := FormatPushNotification(1, "", testTime)

	if !strings.Contains(result, "Файлов: 1") {
		t.Errorf("expected file count in output:\n%s", result)
	}
}

func TestFormatPushNotification_TruncatesLongList(t *testing.T) {
	var names []string
	for i := 0; i < 15; i++ {
		names = append(names, "file"+string(rune('a'+i))+".md")
	}
	result := FormatPushNotification(15, strings.Join(names, "\n"), testTime)

	if !strings.Contains(result, "…и ещё 5") {
		t.Errorf("expected truncation message in output:\n%s", result)
	}
}

func TestFormatPushNotification_CyrillicNames(t *testing.T) {
	fileNames := "00 Inbox/Домашка по английскому.md"
	result := FormatPushNotification(1, fileNames, testTime)

	if !strings.Contains(result, "Домашка по английскому.md") {
		t.Errorf("expected cyrillic filename in output:\n%s", result)
	}
}

func TestFormatConflictAlert(t *testing.T) {
	result := FormatConflictAlert("non-fast-forward", testTime)

	checks := []string{
		"🚨",
		"конфликт",
		"05.04.2026 14:30",
		"ручное вмешательство",
	}
	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output:\n%s", want, result)
		}
	}
}
