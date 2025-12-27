package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/b0d/solar-forecast/internal/domain"
)

// LoadConfig loads configuration from application.properties file
func LoadConfig(configPath string) (*domain.Config, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(configPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, configPath[1:])
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	config := &domain.Config{
		// Set defaults
		ProductionAlertThresholdKW: 2.0,
		DurationThresholdHours:     6,
		DaylightGHIThreshold:       domain.DefaultDaylightGHIThreshold,
		RatedCapacityKW:            5.0,
		InverterEfficiency:         0.97,
		TempCoefficient:            -0.4,
		ChartDisplayHours:          domain.DefaultChartDisplayHours,
		AlertAnalysisHours:         domain.DefaultAlertAnalysisHours,
		NightCompressionFactor:     domain.DefaultNightCompressionFactor,
		APIRetryAttempts:           3,
		APIRetryDelaySeconds:       5,
		APITimeoutSeconds:          10,
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove trailing comments
		if idx := strings.Index(value, "#"); idx != -1 {
			value = strings.TrimSpace(value[:idx])
		}

		switch key {
		case "latitude":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.Latitude = v
			}
		case "longitude":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.Longitude = v
			}
		case "production_alert_threshold_kw":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.ProductionAlertThresholdKW = v
			}
		case "duration_threshold_hours":
			if v, err := strconv.Atoi(value); err == nil {
				config.DurationThresholdHours = v
			}
		// analysis_window_start and analysis_window_end are deprecated
		// daylight detection now uses GHI threshold instead of fixed hours
		case "daylight_ghi_threshold":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.DaylightGHIThreshold = v
			}
		case "rated_capacity_kw":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.RatedCapacityKW = v
			}
		// panel_efficiency is deprecated - rated_capacity_kw already includes it
		case "inverter_efficiency":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.InverterEfficiency = v
			}
		case "temp_coefficient":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.TempCoefficient = v
			}
		case "gmail_app_password":
			config.GmailAppPassword = value
		case "gmail_sender":
			config.GmailSender = value
		case "recipient_email":
			config.RecipientEmail = value
		// daytime_start_hour and daytime_end_hour are deprecated
		// sunrise/sunset is now calculated automatically from coordinates
		case "daytime_start_hour", "daytime_end_hour":
			// Ignored - kept for backwards compatibility
		case "chart_display_hours":
			if v, err := strconv.Atoi(value); err == nil {
				config.ChartDisplayHours = v
			}
		case "alert_analysis_hours":
			if v, err := strconv.Atoi(value); err == nil {
				config.AlertAnalysisHours = v
			}
		case "night_compression_factor":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.NightCompressionFactor = v
			}
		case "api_retry_attempts":
			if v, err := strconv.Atoi(value); err == nil {
				config.APIRetryAttempts = v
			}
		case "api_retry_delay_seconds":
			if v, err := strconv.Atoi(value); err == nil {
				config.APIRetryDelaySeconds = v
			}
		case "api_timeout_seconds":
			if v, err := strconv.Atoi(value); err == nil {
				config.APITimeoutSeconds = v
			}
		case "pushover_user_key":
			config.PushoverUserKey = value
		case "pushover_api_token":
			config.PushoverAPIToken = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(config)

	// Validate required fields
	if config.GmailAppPassword == "" || config.GmailAppPassword == "YOUR_GMAIL_APP_PASSWORD_HERE" {
		return nil, fmt.Errorf("gmail_app_password not configured - please set in config file or SOLAR_GMAIL_APP_PASSWORD env var")
	}
	if config.GmailSender == "" || strings.Contains(config.GmailSender, "your-email@gmail.com") {
		return nil, fmt.Errorf("gmail_sender not properly configured - please set in config file or SOLAR_GMAIL_SENDER env var")
	}
	if config.RecipientEmail == "" || strings.Contains(config.RecipientEmail, "recipient@example.com") {
		return nil, fmt.Errorf("recipient_email not configured - please set in config file or SOLAR_RECIPIENT_EMAIL env var")
	}
	// Validate coordinates
	if config.Latitude == 0 && config.Longitude == 0 {
		return nil, fmt.Errorf("latitude and longitude must be configured")
	}
	if config.Latitude < -90 || config.Latitude > 90 {
		return nil, fmt.Errorf("latitude must be between -90 and 90, got %.6f", config.Latitude)
	}
	if config.Longitude < -180 || config.Longitude > 180 {
		return nil, fmt.Errorf("longitude must be between -180 and 180, got %.6f", config.Longitude)
	}

	// Validate numeric thresholds
	if config.ProductionAlertThresholdKW <= 0 {
		return nil, fmt.Errorf("production_alert_threshold_kw must be positive, got %.2f", config.ProductionAlertThresholdKW)
	}
	if config.DurationThresholdHours < 1 {
		return nil, fmt.Errorf("duration_threshold_hours must be at least 1, got %d", config.DurationThresholdHours)
	}
	if config.RatedCapacityKW <= 0 {
		return nil, fmt.Errorf("rated_capacity_kw must be positive, got %.2f", config.RatedCapacityKW)
	}
	if config.InverterEfficiency <= 0 || config.InverterEfficiency > 1 {
		return nil, fmt.Errorf("inverter_efficiency must be between 0 and 1, got %.2f", config.InverterEfficiency)
	}
	if config.DaylightGHIThreshold < 0 {
		return nil, fmt.Errorf("daylight_ghi_threshold must be non-negative, got %.2f", config.DaylightGHIThreshold)
	}
	if config.ChartDisplayHours < 1 {
		return nil, fmt.Errorf("chart_display_hours must be at least 1, got %d", config.ChartDisplayHours)
	}
	if config.AlertAnalysisHours < 1 {
		return nil, fmt.Errorf("alert_analysis_hours must be at least 1, got %d", config.AlertAnalysisHours)
	}
	if config.NightCompressionFactor < 0 || config.NightCompressionFactor > 1 {
		return nil, fmt.Errorf("night_compression_factor must be between 0 and 1, got %.2f", config.NightCompressionFactor)
	}

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(config *domain.Config) {
	// Test mode override (for make mail command)
	if os.Getenv("SOLAR_TEST_MODE") == "1" {
		config.TestMode = true
		config.ProductionAlertThresholdKW = 5.0
		config.DurationThresholdHours = 1
		fmt.Println("[TEST MODE] Using lowered thresholds: 5.0 kW, 1 hour")
	}

	// Sensitive credentials
	if v := os.Getenv("SOLAR_GMAIL_APP_PASSWORD"); v != "" {
		config.GmailAppPassword = v
	}
	if v := os.Getenv("SOLAR_GMAIL_SENDER"); v != "" {
		config.GmailSender = v
	}
	if v := os.Getenv("SOLAR_RECIPIENT_EMAIL"); v != "" {
		config.RecipientEmail = v
	}
	if v := os.Getenv("SOLAR_PUSHOVER_USER_KEY"); v != "" {
		config.PushoverUserKey = v
	}
	if v := os.Getenv("SOLAR_PUSHOVER_API_TOKEN"); v != "" {
		config.PushoverAPIToken = v
	}

	// Threshold overrides (for testing or adjustment)
	if v := os.Getenv("SOLAR_PRODUCTION_THRESHOLD_KW"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			config.ProductionAlertThresholdKW = val
		}
	}
	if v := os.Getenv("SOLAR_DURATION_THRESHOLD_HOURS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			config.DurationThresholdHours = val
		}
	}
}
