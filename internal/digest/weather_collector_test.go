package digest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWeatherCollector_Collect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"daily": {
				"temperature_2m_max": [18.5],
				"temperature_2m_min": [8.2],
				"apparent_temperature_max": [16.0],
				"apparent_temperature_min": [5.5],
				"precipitation_sum": [2.3],
				"precipitation_probability_max": [75],
				"weather_code": [61]
			}
		}`))
	}))
	defer srv.Close()

	wc := NewWeatherCollector(52.52, 13.41, "Europe/Berlin")
	wc.client = srv.Client()

	// Override URL by replacing the client transport.
	origCollect := wc.Collect
	_ = origCollect

	// Test via direct HTTP server — create a collector that hits the test server.
	tc := &testWeatherCollector{url: srv.URL, client: srv.Client()}
	section, err := tc.collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if section == nil {
		t.Fatal("expected section, got nil")
	}

	checks := []string{"Погода", "Дождь", "18", "8", "Ощущается", "2.3", "75%"}
	for _, want := range checks {
		if !strings.Contains(section.Header+section.Body, want) {
			t.Errorf("expected %q in output:\n%s\n%s", want, section.Header, section.Body)
		}
	}
}

func TestWeatherCollector_NoPrecipitation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"daily": {
				"temperature_2m_max": [25.0],
				"temperature_2m_min": [15.0],
				"apparent_temperature_max": [24.0],
				"apparent_temperature_min": [14.0],
				"precipitation_sum": [0.0],
				"precipitation_probability_max": [0],
				"weather_code": [0]
			}
		}`))
	}))
	defer srv.Close()

	tc := &testWeatherCollector{url: srv.URL, client: srv.Client()}
	section, err := tc.collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(section.Body, "Ясно") {
		t.Errorf("expected 'Ясно' in output:\n%s", section.Body)
	}
	if !strings.Contains(section.Body, "Без осадков") {
		t.Errorf("expected 'Без осадков' in output:\n%s", section.Body)
	}
}

func TestWeatherEmoji(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "☀️"},
		{2, "⛅"},
		{45, "🌫"},
		{61, "🌧"},
		{71, "🌨"},
		{95, "⛈"},
	}
	for _, tt := range tests {
		got := weatherEmoji(tt.code)
		if got != tt.want {
			t.Errorf("weatherEmoji(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// testWeatherCollector is a helper that hits a test HTTP server with the same parsing logic.
type testWeatherCollector struct {
	url    string
	client *http.Client
}

func (tc *testWeatherCollector) collect(ctx context.Context) (*Section, error) {
	wc := NewWeatherCollector(52.52, 13.41, "Europe/Berlin")
	wc.client = tc.client

	// Monkey-patch: override the Collect method by calling the HTTP endpoint directly.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := tc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Re-use the collector's parsing by feeding the response through the same code path.
	// Instead, we just call the real Collect but need to override the URL.
	// Since we can't easily override the URL, we'll parse inline.
	_ = wc
	_ = resp

	// Simpler approach: just use a RoundTripper that redirects to our test server.
	wc.client = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &redirectTransport{url: tc.url, wrapped: http.DefaultTransport},
	}
	return wc.Collect(ctx, time.Now())
}

type redirectTransport struct {
	url     string
	wrapped http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, rt.url, req.Body)
	return rt.wrapped.RoundTrip(newReq)
}
