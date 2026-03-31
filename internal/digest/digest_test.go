package digest

import (
	"context"
	"strings"
	"testing"
	"time"
)

type staticCollector struct {
	section *Section
	err     error
}

func (s *staticCollector) Collect(_ context.Context, _ time.Time) (*Section, error) {
	return s.section, s.err
}

func TestEngine_Build_Empty(t *testing.T) {
	e := NewEngine()
	msg, err := e.Build(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if msg != "" {
		t.Errorf("expected empty, got: %s", msg)
	}
}

func TestEngine_Build_SingleCollector(t *testing.T) {
	e := NewEngine()
	e.Register(&staticCollector{section: &Section{
		Header: "📝 *Tasks*",
		Body:   "some tasks here",
	}})

	msg, err := e.Build(context.Background(), time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Доброе утро") {
		t.Error("expected greeting")
	}
	if !strings.Contains(msg, "📝 *Tasks*") {
		t.Error("expected section header")
	}
	if !strings.Contains(msg, "some tasks here") {
		t.Error("expected section body")
	}
}

func TestEngine_Build_SkipsNilSections(t *testing.T) {
	e := NewEngine()
	e.Register(&staticCollector{section: nil})
	e.Register(&staticCollector{section: &Section{Header: "H", Body: "B"}})

	msg, err := e.Build(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "B") {
		t.Error("expected content from second collector")
	}
}

func TestEngine_Build_MultipleCollectors(t *testing.T) {
	e := NewEngine()
	e.Register(&staticCollector{section: &Section{Header: "🅰️ *First*", Body: "aaa"}})
	e.Register(&staticCollector{section: &Section{Header: "🅱️ *Second*", Body: "bbb"}})

	msg, err := e.Build(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	aIdx := strings.Index(msg, "First")
	bIdx := strings.Index(msg, "Second")
	if aIdx > bIdx {
		t.Error("collectors should appear in registration order")
	}
}
