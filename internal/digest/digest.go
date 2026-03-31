package digest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Section is what a Collector returns — a header and a formatted body.
type Section struct {
	Header string
	Body   string
}

// Collector gathers data and produces a section for the morning digest.
type Collector interface {
	// Collect returns a section for the digest. Return nil to skip this collector.
	Collect(ctx context.Context, now time.Time) (*Section, error)
}

// Engine holds registered collectors and builds the morning digest message.
type Engine struct {
	collectors []Collector
}

// NewEngine creates a new digest engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Register adds a collector to the engine.
func (e *Engine) Register(c Collector) {
	e.collectors = append(e.collectors, c)
}

// Build runs all collectors and assembles the digest message.
// Returns empty string if no collector produced output.
func (e *Engine) Build(ctx context.Context, now time.Time) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("☀️ *Доброе утро!*\n_%s_\n", now.Format("02 January 2006")))

	hasContent := false
	for _, c := range e.collectors {
		section, err := c.Collect(ctx, now)
		if err != nil {
			return "", fmt.Errorf("collector error: %w", err)
		}
		if section == nil || section.Body == "" {
			continue
		}
		hasContent = true
		b.WriteString(fmt.Sprintf("\n%s\n%s", section.Header, section.Body))
	}

	if !hasContent {
		return "", nil
	}
	return b.String(), nil
}
