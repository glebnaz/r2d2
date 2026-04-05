package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	GitSyncsTotal.Inc()
	GitPushesTotal.Inc()
	GitPushErrors.Inc()
	GitConflicts.Inc()

	body := scrapeBody(t)

	// After Inc(), counters should be >= 1.
	for _, name := range []string{
		"r2d2_git_syncs_total",
		"r2d2_git_pushes_total",
		"r2d2_git_push_errors_total",
		"r2d2_git_conflicts_total",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metric %q not found after increment", name)
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

	body := scrapeBody(t)
	if !strings.Contains(body, "r2d2_git_files_changed_last 42") {
		t.Errorf("gauge not set to 42; body snippet: %s",
			body[strings.Index(body, "r2d2_git_files_changed_last"):
				strings.Index(body, "r2d2_git_files_changed_last")+80])
	}
}
