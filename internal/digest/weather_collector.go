package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WeatherCollector fetches weather from Open-Meteo and produces a digest section.
type WeatherCollector struct {
	lat, lon float64
	timezone string
	client   *http.Client
}

// NewWeatherCollector creates a weather collector for the given coordinates.
func NewWeatherCollector(lat, lon float64, timezone string) *WeatherCollector {
	return &WeatherCollector{
		lat:      lat,
		lon:      lon,
		timezone: timezone,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type openMeteoResponse struct {
	Daily struct {
		TemperatureMax    []float64 `json:"temperature_2m_max"`
		TemperatureMin    []float64 `json:"temperature_2m_min"`
		ApparentTempMax   []float64 `json:"apparent_temperature_max"`
		ApparentTempMin   []float64 `json:"apparent_temperature_min"`
		PrecipitationSum  []float64 `json:"precipitation_sum"`
		PrecipitationProb []int     `json:"precipitation_probability_max"`
		WeatherCode       []int     `json:"weather_code"`
	} `json:"daily"`
}

// Collect fetches today's weather and returns a formatted digest section.
func (w *WeatherCollector) Collect(ctx context.Context, _ time.Time) (*Section, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&daily=temperature_2m_max,temperature_2m_min,apparent_temperature_max,apparent_temperature_min,precipitation_sum,precipitation_probability_max,weather_code"+
			"&timezone=%s&forecast_days=1",
		w.lat, w.lon, w.timezone,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating weather request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching weather: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading weather response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned %d: %s", resp.StatusCode, body)
	}

	var data openMeteoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing weather response: %w", err)
	}

	if len(data.Daily.TemperatureMax) == 0 {
		return nil, nil
	}

	tempMax := data.Daily.TemperatureMax[0]
	tempMin := data.Daily.TemperatureMin[0]
	feelsMax := data.Daily.ApparentTempMax[0]
	feelsMin := data.Daily.ApparentTempMin[0]
	precip := data.Daily.PrecipitationSum[0]
	code := data.Daily.WeatherCode[0]

	var precipProb int
	if len(data.Daily.PrecipitationProb) > 0 {
		precipProb = data.Daily.PrecipitationProb[0]
	}

	emoji := weatherEmoji(code)
	desc := weatherDescription(code)

	var b strings.Builder
	fmt.Fprintf(&b, "\n%s %s\n", emoji, desc)
	fmt.Fprintf(&b, "🌡 %+.0f…%+.0f°C\n", tempMin, tempMax)
	fmt.Fprintf(&b, "🤔 Ощущается: %+.0f…%+.0f°C\n", feelsMin, feelsMax)

	if precip > 0 {
		fmt.Fprintf(&b, "🌧 Осадки: %.1f мм", precip)
	} else {
		b.WriteString("☂️ Без осадков")
	}
	if precipProb > 0 {
		fmt.Fprintf(&b, " (%d%%)", precipProb)
	}
	b.WriteString("\n")

	return &Section{
		Header: "🌤 *Погода*",
		Body:   b.String(),
	}, nil
}

// weatherEmoji returns an emoji for WMO weather code.
func weatherEmoji(code int) string {
	switch {
	case code == 0:
		return "☀️"
	case code <= 3:
		return "⛅"
	case code <= 49:
		return "🌫"
	case code <= 59:
		return "🌦"
	case code <= 69:
		return "🌧"
	case code <= 79:
		return "🌨"
	case code <= 82:
		return "🌧"
	case code <= 86:
		return "🌨"
	case code >= 95:
		return "⛈"
	default:
		return "🌡"
	}
}

// weatherDescription returns a Russian description for WMO weather code.
func weatherDescription(code int) string {
	switch {
	case code == 0:
		return "Ясно"
	case code == 1:
		return "Преимущественно ясно"
	case code == 2:
		return "Переменная облачность"
	case code == 3:
		return "Пасмурно"
	case code <= 49:
		return "Туман"
	case code <= 59:
		return "Морось"
	case code <= 69:
		return "Дождь"
	case code <= 79:
		return "Снег"
	case code <= 82:
		return "Ливень"
	case code <= 86:
		return "Снегопад"
	case code >= 95:
		return "Гроза"
	default:
		return "Неизвестно"
	}
}
