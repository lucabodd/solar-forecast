package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b0d/solar-forecast/internal/adapters"
	"github.com/b0d/solar-forecast/internal/config"
	"github.com/b0d/solar-forecast/internal/domain"
)

const version = "1.0.1"

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config/application.properties", "Path to configuration file")
	stateDir := flag.String("state", "~/.solar-forecast", "Directory for state files")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Initialize logger
	logger := adapters.NewSimpleLogger(*debug)

	logger.Info("Solar Forecast Warning System started", "version", "1.0")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Configuration loaded successfully",
		"latitude", cfg.Latitude,
		"longitude", cfg.Longitude,
		"production_alert_threshold_kw", cfg.ProductionAlertThresholdKW,
		"duration_threshold_hours", cfg.DurationThresholdHours,
		"rated_capacity_kw", cfg.RatedCapacityKW,
	)

	// Expand state directory path
	stateFilePath := filepath.Join(expandPath(*stateDir), "alert_state.json")
	logger.Info("State file path", "path", stateFilePath)

	// Initialize adapters
	weatherProvider := adapters.NewOpenMeteoAdapter(cfg, logger)
	emailNotifier := adapters.NewGmailAdapter(cfg, logger)
	pushNotifier := adapters.NewPushoverAdapter(cfg, logger)
	stateRepository := adapters.NewFileStateAdapter(stateFilePath, logger)

	// Create service
	service := domain.NewSolarForecastService(
		cfg,
		weatherProvider,
		emailNotifier,
		pushNotifier,
		stateRepository,
		logger,
	)

	// Run check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := service.CheckAndAlert(ctx); err != nil {
		logger.Error("Service error", "error", err.Error())
		os.Exit(1)
	}

	logger.Info("Check completed successfully")
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if path == "~" || path == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
