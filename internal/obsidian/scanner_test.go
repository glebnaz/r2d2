package obsidian

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

var testLoc = mustLoadLocation("Europe/Moscow")

func createTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanVault_BasicTask(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "Buy groceries.md", `---
type: Task
status: todo
due: 2025-06-15
priority: high
project: personal
---
Some body text
`)

	tasks, err := ScanVault(dir, []string{"todo", "in-progress"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.Title != "Buy groceries" {
		t.Errorf("title = %q, want %q", task.Title, "Buy groceries")
	}
	if task.HasTime {
		t.Error("expected HasTime=false for date-only due")
	}
	expected := time.Date(2025, 6, 15, 0, 0, 0, 0, testLoc)
	if !task.Due.Equal(expected) {
		t.Errorf("due = %v, want %v", task.Due, expected)
	}
	if task.Priority != "high" {
		t.Errorf("priority = %q, want %q", task.Priority, "high")
	}
	if task.Project != "personal" {
		t.Errorf("project = %q, want %q", task.Project, "personal")
	}
}

func TestScanVault_DatetimeDue(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "Meeting.md", `---
type: Task
status: todo
due: 2025-06-15T14:30
---
`)

	tasks, err := ScanVault(dir, []string{"todo"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if !task.HasTime {
		t.Error("expected HasTime=true for datetime due")
	}
	expected := time.Date(2025, 6, 15, 14, 30, 0, 0, testLoc)
	if !task.Due.Equal(expected) {
		t.Errorf("due = %v, want %v", task.Due, expected)
	}
}

func TestScanVault_FiltersNonTask(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "note.md", `---
type: Note
status: todo
due: 2025-06-15
---
`)

	tasks, err := ScanVault(dir, []string{"todo"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestScanVault_FiltersWrongStatus(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "done-task.md", `---
type: Task
status: done
due: 2025-06-15
---
`)

	tasks, err := ScanVault(dir, []string{"todo", "in-progress"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestScanVault_SkipsNoDue(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "no-due.md", `---
type: Task
status: todo
---
`)

	tasks, err := ScanVault(dir, []string{"todo"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestScanVault_SkipsNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "plain.md", `Just a regular note without frontmatter.`)

	tasks, err := ScanVault(dir, []string{"todo"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestScanVault_RecursiveSubdirs(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "sub/deep/task.md", `---
type: Task
status: in-progress
due: 2025-07-01
---
`)

	tasks, err := ScanVault(dir, []string{"in-progress"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "task" {
		t.Errorf("title = %q, want %q", tasks[0].Title, "task")
	}
}

func TestScanVault_MultipleTasks(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "task1.md", `---
type: Task
status: todo
due: 2025-06-15
---
`)
	createTestFile(t, dir, "task2.md", `---
type: Task
status: block
due: 2025-06-16T10:00
---
`)
	createTestFile(t, dir, "not-a-task.md", `---
type: Note
due: 2025-06-15
---
`)

	tasks, err := ScanVault(dir, []string{"todo", "block"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestScanVault_InvalidDueSkipped(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, dir, "bad-date.md", `---
type: Task
status: todo
due: not-a-date
---
`)

	tasks, err := ScanVault(dir, []string{"todo"}, testLoc)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestParseDue(t *testing.T) {
	tests := []struct {
		input   string
		wantTime bool
		wantErr bool
	}{
		{"2025-06-15", false, false},
		{"2025-06-15T14:30", true, false},
		{"not-a-date", false, true},
		{"2025/06/15", false, true},
		{"", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, hasTime, err := parseDue(tt.input, testLoc)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDue(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && hasTime != tt.wantTime {
				t.Errorf("parseDue(%q) hasTime = %v, want %v", tt.input, hasTime, tt.wantTime)
			}
		})
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("no frontmatter here"), 0o644); err != nil {
		t.Fatal(err)
	}

	fm, err := parseFrontmatter(path)
	if err != nil {
		t.Fatal(err)
	}
	if fm != nil {
		t.Error("expected nil frontmatter")
	}
}

func TestParseFrontmatter_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	fm, err := parseFrontmatter(path)
	if err != nil {
		t.Fatal(err)
	}
	if fm != nil {
		t.Error("expected nil frontmatter")
	}
}
