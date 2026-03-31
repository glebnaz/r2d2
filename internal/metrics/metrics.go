package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Business metrics.

	TasksScanned = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "r2d2_tasks_scanned_total",
		Help: "Number of tasks found in the last vault scan.",
	})

	TasksDueToday = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "r2d2_tasks_due_today",
		Help: "Number of tasks due today (date-only).",
	})

	TasksOverdue = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "r2d2_tasks_overdue",
		Help: "Number of overdue tasks.",
	})

	DigestsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "r2d2_digests_sent_total",
		Help: "Total number of morning digests sent.",
	})

	TimedRemindersSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "r2d2_timed_reminders_sent_total",
		Help: "Total number of timed reminders sent.",
	})

	NotificationErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "r2d2_notification_errors_total",
		Help: "Total number of failed notification sends.",
	})

	VaultScans = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "r2d2_vault_scans_total",
		Help: "Total number of vault scans performed.",
	})

	VaultScanErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "r2d2_vault_scan_errors_total",
		Help: "Total number of failed vault scans.",
	})

	VaultScanDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "r2d2_vault_scan_duration_seconds",
		Help:    "Duration of vault scans.",
		Buckets: prometheus.DefBuckets,
	})

	CollectorDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "r2d2_collector_duration_seconds",
		Help:    "Duration of digest collector execution.",
		Buckets: prometheus.DefBuckets,
	}, []string{"collector"})
)

func init() {
	prometheus.MustRegister(
		TasksScanned,
		TasksDueToday,
		TasksOverdue,
		DigestsSent,
		TimedRemindersSent,
		NotificationErrors,
		VaultScans,
		VaultScanErrors,
		VaultScanDuration,
		CollectorDuration,
	)
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
