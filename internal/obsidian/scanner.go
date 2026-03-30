package obsidian

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Task represents a parsed Obsidian task with relevant frontmatter fields.
type Task struct {
	Title    string
	Due      time.Time
	HasTime  bool
	Priority string
	Project  string
	FilePath string
}

// frontmatter represents the YAML frontmatter of an Obsidian note.
type frontmatter struct {
	Type     string `yaml:"type"`
	Status   string `yaml:"status"`
	Due      string `yaml:"due"`
	Priority string `yaml:"priority"`
	Project  string `yaml:"project"`
}

// ScanVault recursively scans the vault at vaultPath for .md files,
// parses their frontmatter, and returns tasks matching the filter criteria.
func ScanVault(vaultPath string, reminderStatuses []string, loc *time.Location) ([]Task, error) {
	statusSet := make(map[string]bool, len(reminderStatuses))
	for _, s := range reminderStatuses {
		statusSet[s] = true
	}

	var tasks []Task

	err := filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		fm, err := parseFrontmatter(path)
		if err != nil {
			// Skip files with unparseable frontmatter.
			return nil
		}
		if fm == nil {
			return nil
		}

		if fm.Type != "Task" {
			return nil
		}
		if !statusSet[fm.Status] {
			return nil
		}
		if fm.Due == "" {
			return nil
		}

		due, hasTime, err := parseDue(fm.Due, loc)
		if err != nil {
			return nil
		}

		title := strings.TrimSuffix(info.Name(), ".md")

		tasks = append(tasks, Task{
			Title:    title,
			Due:      due,
			HasTime:  hasTime,
			Priority: fm.Priority,
			Project:  fm.Project,
			FilePath: path,
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning vault: %w", err)
	}

	return tasks, nil
}

// parseFrontmatter reads a file and extracts YAML frontmatter between --- delimiters.
// Returns nil if no frontmatter is found.
func parseFrontmatter(path string) (*frontmatter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)

	// First line must be "---"
	if !scanner.Scan() {
		return nil, nil
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, nil
	}

	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return nil, nil
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &fm); err != nil {
		return nil, err
	}

	return &fm, nil
}

// parseDue parses a due string as either YYYY-MM-DD or YYYY-MM-DDTHH:mm.
func parseDue(due string, loc *time.Location) (time.Time, bool, error) {
	if t, err := time.ParseInLocation("2006-01-02T15:04", due, loc); err == nil {
		return t, true, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", due, loc); err == nil {
		return t, false, nil
	}
	return time.Time{}, false, fmt.Errorf("invalid due format: %s", due)
}
