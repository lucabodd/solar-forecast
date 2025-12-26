package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/b0d/solar-forecast/internal/domain"
)

// OpenMeteoAdapter implements WeatherForecastProvider using Open-Meteo API
type OpenMeteoAdapter struct {
	httpClient      *http.Client
	retryAttempts   int
	retryDelay      time.Duration
	logger          domain.Logger
}

// OpenMeteoResponse represents the API response structure
type OpenMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Hourly    struct {
		Time                     []string  `json:"time"`
		Temperature2m            []float64 `json:"temperature_2m"`
		CloudCover               []int     `json:"cloud_cover"`
		ShortwaveRadiation       []float64 `json:"shortwave_radiation"`
		RelativeHumidity2m       []int     `json:"relative_humidity_2m"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
	} `json:"hourly"`
}

// NewOpenMeteoAdapter creates a new Open-Meteo adapter
func NewOpenMeteoAdapter(config *domain.Config, logger domain.Logger) *OpenMeteoAdapter {
	return &OpenMeteoAdapter{
		httpClient: &http.Client{
			Timeout: time.Duration(config.APITimeoutSeconds) * time.Second,
		},
		retryAttempts: config.APIRetryAttempts,
		retryDelay:    time.Duration(config.APIRetryDelaySeconds) * time.Second,
		logger:        logger,
	}
}

// GetForecast fetches 7-day weather forecast from Open-Meteo API with retries
func (a *OpenMeteoAdapter) GetForecast(ctx context.Context, latitude, longitude float64) (*domain.ForecastData, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.2f&longitude=%.2f&hourly=temperature_2m,cloud_cover,shortwave_radiation,relative_humidity_2m,precipitation_probability&forecast_days=7&timezone=auto",
		latitude, longitude,
	)

	var lastErr error
	for attempt := 0; attempt < a.retryAttempts; attempt++ {
		if attempt > 0 {
			a.logger.Info("Retrying Open-Meteo API", "attempt", attempt, "delay_seconds", a.retryDelay.Seconds())
			select {
			case <-time.After(a.retryDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := a.httpClient.Do(a.createRequest(ctx, url))
		if err != nil {
			lastErr = err
			a.logger.Error("Failed to fetch from Open-Meteo", "error", err.Error(), "attempt", attempt+1)
			continue
		}

		data, err := a.parseResponse(resp)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			a.logger.Error("Failed to parse Open-Meteo response", "error", err.Error(), "attempt", attempt+1)
			continue
		}

		a.logger.Info("Successfully fetched forecast from Open-Meteo", "hours", len(data.Hours))
		return data, nil
	}

	return nil, fmt.Errorf("failed to get forecast after %d attempts: %w", a.retryAttempts, lastErr)
}

// createRequest creates an HTTP request with context
func (a *OpenMeteoAdapter) createRequest(ctx context.Context, url string) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "SolarForecast/1.0")
	return req
}

// parseResponse parses the Open-Meteo API response
func (a *OpenMeteoAdapter) parseResponse(resp *http.Response) (*domain.ForecastData, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp OpenMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return a.buildForecastData(apiResp)
}

// buildForecastData converts API response to domain model
func (a *OpenMeteoAdapter) buildForecastData(apiResp OpenMeteoResponse) (*domain.ForecastData, error) {
	forecast := &domain.ForecastData{
		Hours: make([]domain.ForecastHour, 0),
	}

	// Ensure we have consistent data
	minLen := len(apiResp.Hourly.Time)
	if len(apiResp.Hourly.Temperature2m) < minLen {
		minLen = len(apiResp.Hourly.Temperature2m)
	}
	if len(apiResp.Hourly.CloudCover) < minLen {
		minLen = len(apiResp.Hourly.CloudCover)
	}
	if len(apiResp.Hourly.ShortwaveRadiation) < minLen {
		minLen = len(apiResp.Hourly.ShortwaveRadiation)
	}
	if len(apiResp.Hourly.RelativeHumidity2m) < minLen {
		minLen = len(apiResp.Hourly.RelativeHumidity2m)
	}
	if len(apiResp.Hourly.PrecipitationProbability) < minLen {
		minLen = len(apiResp.Hourly.PrecipitationProbability)
	}

	for i := 0; i < minLen && i < 168; i++ { // Limit to 168 hours (7 days)
		hour, err := time.Parse("2006-01-02T15:04", apiResp.Hourly.Time[i])
		if err != nil {
			a.logger.Error("Failed to parse time", "time_string", apiResp.Hourly.Time[i], "error", err.Error())
			continue
		}

		cloudCover := apiResp.Hourly.CloudCover[i]
		if cloudCover < 0 {
			cloudCover = 0
		}
		if cloudCover > 100 {
			cloudCover = 100
		}

		humidity := apiResp.Hourly.RelativeHumidity2m[i]
		if humidity < 0 {
			humidity = 0
		}
		if humidity > 100 {
			humidity = 100
		}

		precipProb := apiResp.Hourly.PrecipitationProbability[i]
		if precipProb < 0 {
			precipProb = 0
		}
		if precipProb > 100 {
			precipProb = 100
		}

		forecast.Hours = append(forecast.Hours, domain.ForecastHour{
			Hour:                        hour,
			Temperature:                 apiResp.Hourly.Temperature2m[i],
			CloudCover:                  cloudCover,
			GlobalHorizontalIrradiance: apiResp.Hourly.ShortwaveRadiation[i],
			RelativeHumidity:            humidity,
			PrecipitationProbability:    precipProb,
		})
	}

	if len(forecast.Hours) == 0 {
		return nil, fmt.Errorf("no valid forecast hours extracted")
	}

	return forecast, nil
}
