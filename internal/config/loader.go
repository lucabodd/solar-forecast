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
		AnalysisWindowStart:        10,
		AnalysisWindowEnd:          16,
		RatedCapacityKW:           5.0,
		PanelEfficiency:           0.20,
		InverterEfficiency:        0.97,
		TempCoefficient:           -0.4,
		DaytimeStartHour:          6,
		DaytimeEndHour:            18,
		APIRetryAttempts:          3,
		APIRetryDelaySeconds:      5,
		APITimeoutSeconds:         10,
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
		case "analysis_window_start":
			if v, err := strconv.Atoi(value); err == nil {
				config.AnalysisWindowStart = v
			}
		case "analysis_window_end":
			if v, err := strconv.Atoi(value); err == nil {
				config.AnalysisWindowEnd = v
			}
		case "rated_capacity_kw":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.RatedCapacityKW = v
			}
		case "panel_efficiency":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				config.PanelEfficiency = v
				// Note: This value is deprecated and not used in calculations
				// The rated capacity already includes panel efficiency
			}
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
		case "daytime_start_hour":
			if v, err := strconv.Atoi(value); err == nil {
				config.DaytimeStartHour = v
			}
		case "daytime_end_hour":
			if v, err := strconv.Atoi(value); err == nil {
				config.DaytimeEndHour = v
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
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Validate required fields
	if config.GmailAppPassword == "" || config.GmailAppPassword == "xxxx xxxx xxxx xxxx" {
		return nil, fmt.Errorf("gmail_app_password not configured (set to xxxx xxxx xxxx xxxx)")
	}
	if config.GmailSender == "" || strings.Contains(config.GmailSender, "@gmail.com") == false {
		return nil, fmt.Errorf("gmail_sender not properly configured")
	}
	if config.RecipientEmail == "" {
		return nil, fmt.Errorf("recipient_email not configured")
	}
	if config.Latitude == 0 || config.Longitude == 0 {
		return nil, fmt.Errorf("latitude and longitude must be configured")
	}

	return config, nil
}
