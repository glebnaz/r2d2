package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func scrapeBody(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics handler returned %d", rec.Code)
	}
	return rec.Body.String()
}

func TestGitSyncMetricsRegistered(t *testing.T) {
	body := scrapeBody(t)

	expected := []string{
		"r2d2_git_syncs_total",
		"r2d2_git_pushes_total",
		"r2d2_git_push_errors_total",
		"r2d2_git_conflicts_total",
		"r2d2_git_sync_duration_seconds",
		"r2d2_git_files_changed_last",
	}

	for _, name := range expected {
		if !strings.Contains(body, name) {
			t.Errorf("metric %q not found in /metrics output", name)
		}
	}
}

func TestGitSyncCounterIncrement(t *testing.T) {
	counters := []struct {
		name    string
		counter interface{ Inc() }
		getter  func() float64
	}{
		{"r2d2_git_syncs_total", GitSyncsTotal, func() float64 { return testutil.ToFloat64(GitSyncsTotal) }},
		{"r2d2_git_pushes_total", GitPushesTotal, func() float64 { return testutil.ToFloat64(GitPushesTotal) }},
		{"r2d2_git_push_errors_total", GitPushErrors, func() float64 { return testutil.ToFloat64(GitPushErrors) }},
		{"r2d2_git_conflicts_total", GitConflicts, func() float64 { return testutil.ToFloat64(GitConflicts) }},
	}

	for _, c := range counters {
		before := c.getter()
		c.counter.Inc()
		after := c.getter()
		if after != before+1 {
			t.Errorf("%s: expected value to increase by 1 (before=%v, after=%v)", c.name, before, after)
		}
	}
}

func TestGitSyncDurationObserve(t *testing.T) {
	GitSyncDuration.Observe(0.5)

	body := scrapeBody(t)
	if !strings.Contains(body, "r2d2_git_sync_duration_seconds_count") {
		t.Error("histogram count not found after Observe")
	}
}

func TestGitFilesChangedGauge(t *testing.T) {
	GitFilesChanged.Set(42)

	val := testutil.ToFloat64(GitFilesChanged)
	if val != 42 {
		t.Errorf("gauge = %v, want 42", val)
	}

	// Also verify it appears in scraped output.
	body := scrapeBody(t)
	expected := fmt.Sprintf("r2d2_git_files_changed_last %v", val)
	if !strings.Contains(body, expected) {
		t.Errorf("gauge not found in /metrics output: %s", body)
	}
}
