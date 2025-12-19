package domain

import (
	"context"
	"fmt"
	"time"
)

// SolarForecastService orchestrates solar forecast checking and alerting
type SolarForecastService struct {
	config              *Config
	weatherProvider     WeatherForecastProvider
	emailNotifier       EmailNotifier
	stateRepository     AlertStateRepository
	logger              Logger
}

// NewSolarForecastService creates a new service instance
func NewSolarForecastService(
	config *Config,
	weatherProvider WeatherForecastProvider,
	emailNotifier EmailNotifier,
	stateRepository AlertStateRepository,
	logger Logger,
) *SolarForecastService {
	return &SolarForecastService{
		config:          config,
		weatherProvider: weatherProvider,
		emailNotifier:   emailNotifier,
		stateRepository: stateRepository,
		logger:          logger,
	}
}

// CheckAndAlert performs the complete check and alert workflow
func (s *SolarForecastService) CheckAndAlert(ctx context.Context) error {
	s.logger.Info("Starting solar forecast check")

	// Reset alert state if new day
	reset, err := s.stateRepository.ResetIfNewDay(ctx)
	if err != nil {
		s.logger.Error("Failed to check/reset daily state", "error", err.Error())
	}
	if reset {
		s.logger.Info("Daily alert state reset")
	}

	// Fetch forecast data
	forecast, err := s.weatherProvider.GetForecast(ctx, s.config.Latitude, s.config.Longitude)
	if err != nil {
		s.logger.Error("Failed to fetch weather forecast", "error", err.Error())
		return fmt.Errorf("failed to fetch forecast: %w", err)
	}

	s.logger.Info("Forecast fetched successfully", "hours", len(forecast.Hours))

	// Analyze forecast for alert conditions
	analysis := s.analyzeForecast(forecast)

	// Log analysis results
	s.logger.Info("Forecast analysis complete",
		"low_production_duration_triggered", analysis.CriteriaTriggered.LowProductionDurationTriggered,
		"consecutive_hours", analysis.ConsecutiveHourCount,
		"first_low_hour", analysis.FirstLowProductionHour.Format("15:04"),
		"last_low_hour", analysis.LastLowProductionHour.Format("15:04"),
	)

	// Check if we should send alert
	if !analysis.CriteriaTriggered.AnyTriggered {
		s.logger.Info("No alert criteria triggered")
		
		// Check if we should send recovery email (conditions have improved)
		shouldSendRecovery, err := s.stateRepository.ShouldSendRecoveryEmail(ctx)
		if err != nil {
			s.logger.Error("Failed to check recovery email status", "error", err.Error())
			return err
		}

		if shouldSendRecovery {
			s.logger.Info("Recovery conditions met - preparing to send recovery email",
				"reason", "alert_previously_sent_and_conditions_improved")

			if err := s.emailNotifier.SendRecoveryEmail(ctx); err != nil {
				s.logger.Error("Failed to send recovery email", "error", err.Error())
				return fmt.Errorf("failed to send recovery email: %w", err)
			}

			s.logger.Info("Recovery email sent successfully")

			if err := s.stateRepository.MarkRecoveryEmailSent(ctx); err != nil {
				s.logger.Error("Failed to mark recovery email as sent", "error", err.Error())
				return fmt.Errorf("failed to mark recovery email sent: %w", err)
			}

			s.logger.Info("Recovery email marked as sent in state file")
		} else {
			s.logger.Debug("Recovery email not needed")
		}
		
		return nil
	}

	// Check if alert was already sent today
	shouldSend, err := s.shouldSendAlert(ctx)
	if err != nil {
		s.logger.Error("Failed to check if alert should be sent", "error", err.Error())
		return err
	}

	if !shouldSend {
		s.logger.Info("Alert already sent today, skipping")
		return nil
	}

	// Send alert email
	if err := s.emailNotifier.SendAlert(ctx, analysis); err != nil {
		s.logger.Error("Failed to send alert email", "error", err.Error())
		return fmt.Errorf("failed to send alert: %w", err)
	}

	// Mark alert as sent
	if err := s.stateRepository.MarkAlertSent(ctx); err != nil {
		s.logger.Error("Failed to mark alert as sent", "error", err.Error())
		return fmt.Errorf("failed to mark alert sent: %w", err)
	}

	s.logger.Info("Alert email sent successfully")
	return nil
}

// analyzeForecast analyzes forecast data against alert criteria
func (s *SolarForecastService) analyzeForecast(forecast *ForecastData) *AlertAnalysis {
	analysis := &AlertAnalysis{
		LowProductionHours: []SolarProduction{},
	}

	// Filter hours to analysis window
	windowHours := s.filterAnalysisWindow(forecast.Hours)
	if len(windowHours) == 0 {
		s.logger.Warn("No hours in analysis window")
		analysis.RecommendedAction = "No data in analysis window. Check again during daytime hours."
		return analysis
	}

	// Calculate solar production for each hour
	productionData := make([]SolarProduction, len(windowHours))
	for i, hour := range windowHours {
		productionData[i] = s.calculateSolarProduction(hour)
	}

	// Evaluate low production duration criterion
	s.evaluateLowProductionDuration(productionData, analysis)

	// Determine if any criterion triggered
	analysis.CriteriaTriggered.AnyTriggered = analysis.CriteriaTriggered.LowProductionDurationTriggered

	if !analysis.CriteriaTriggered.AnyTriggered {
		analysis.RecommendedAction = "Solar production forecast looks normal. No action required."
		return analysis
	}

	// Generate recommendation
	analysis.RecommendedAction = s.generateRecommendation(analysis)

	return analysis
}

// filterAnalysisWindow filters forecast hours to daylight hours only
// Uses GHI (Global Horizontal Irradiance) to determine daylight - more accurate than fixed times
// as it adapts to seasonal changes and actual solar conditions
func (s *SolarForecastService) filterAnalysisWindow(hours []ForecastHour) []ForecastHour {
	filtered := []ForecastHour{}

	// GHI threshold in W/m² to consider as "daylight"
	// Typically sunrise/sunset has GHI around 50-100 W/m²
	// We use 50 W/m² as minimum threshold to capture full daylight period
	const daylightGHIThreshold = 50.0

	for _, hour := range hours {
		// Include hour if there's meaningful solar radiation
		if hour.GlobalHorizontalIrradiance >= daylightGHIThreshold {
			filtered = append(filtered, hour)
		}
	}

	s.logger.Debug("Filtered to daylight hours",
		"total_hours", len(hours),
		"daylight_hours", len(filtered),
		"ghi_threshold", daylightGHIThreshold)

	return filtered
}

// evaluateLowProductionDuration checks if production drops below threshold for 6+ consecutive hours
func (s *SolarForecastService) evaluateLowProductionDuration(production []SolarProduction, analysis *AlertAnalysis) {
	var maxConsecutiveCount int
	var maxConsecutiveStart, maxConsecutiveEnd time.Time
	var maxConsecutiveHours []SolarProduction

	currentConsecutiveCount := 0
	var currentConsecutiveStart time.Time
	var currentConsecutiveHours []SolarProduction

	for _, prod := range production {
		s.logger.Debug("Production hour",
			"hour", prod.Hour.Format("15:04"),
			"production_kw", fmt.Sprintf("%.2f", prod.EstimatedOutputKW),
			"below_threshold", prod.EstimatedOutputKW < s.config.ProductionAlertThresholdKW,
		)

		if prod.EstimatedOutputKW < s.config.ProductionAlertThresholdKW {
			// Below threshold
			if currentConsecutiveCount == 0 {
				currentConsecutiveStart = prod.Hour
			}
			currentConsecutiveCount++
			currentConsecutiveHours = append(currentConsecutiveHours, prod)

			// Check if this is the max so far
			if currentConsecutiveCount > maxConsecutiveCount {
				maxConsecutiveCount = currentConsecutiveCount
				maxConsecutiveStart = currentConsecutiveStart
				maxConsecutiveEnd = prod.Hour
				maxConsecutiveHours = append([]SolarProduction{}, currentConsecutiveHours...)
			}
		} else {
			// Above threshold - reset consecutive count
			currentConsecutiveCount = 0
			currentConsecutiveHours = nil
		}
	}

	s.logger.Debug("Low production duration evaluation",
		"threshold_kw", s.config.ProductionAlertThresholdKW,
		"duration_threshold_hours", s.config.DurationThresholdHours,
		"max_consecutive_hours", maxConsecutiveCount,
		"triggered", maxConsecutiveCount >= s.config.DurationThresholdHours,
	)

	if maxConsecutiveCount >= s.config.DurationThresholdHours {
		analysis.CriteriaTriggered.LowProductionDurationTriggered = true
		analysis.ConsecutiveHourCount = maxConsecutiveCount
		analysis.FirstLowProductionHour = maxConsecutiveStart
		analysis.LastLowProductionHour = maxConsecutiveEnd
		analysis.LowProductionHours = maxConsecutiveHours
	}
}

// calculateSolarProduction estimates solar output for a given hour
func (s *SolarForecastService) calculateSolarProduction(hour ForecastHour) SolarProduction {
	prod := SolarProduction{
		Hour:        hour.Hour,
		CloudCover:  hour.CloudCover,
		Temperature: hour.Temperature,
		GHI:         hour.GlobalHorizontalIrradiance,
	}

	// Formula: P_out = P_rated × (GHI/1000) × η_inverter × temp_adjustment
	//
	// Note: RatedCapacityKW (8.9 kW for 16×560W panels) is the manufacturer's rated
	// output at STC (Standard Test Conditions: 1000 W/m² irradiance, 25°C, AM1.5 spectrum).
	// This rating ALREADY includes the panel's conversion efficiency (~20% silicon),
	// so we only need to adjust for:
	//   1. Actual irradiance vs reference (GHI/1000)
	//   2. Inverter losses (DC to AC conversion)
	//   3. Temperature effects
	//
	// GHI (Global Horizontal Irradiance) from Open-Meteo already accounts for
	// cloud cover, atmospheric conditions, and solar angle.
	//
	// Reference GHI is 1000 W/m² (STC)

	// Normalize GHI to reference (1000 W/m²)
	ghiFactor := hour.GlobalHorizontalIrradiance / 1000.0

	// Temperature adjustment (efficiency decreases with temperature)
	// Assuming reference temp of 25°C
	tempAdjustment := 1.0 - (s.config.TempCoefficient / 100.0 * (hour.Temperature - 25.0))

	// Calculate output (panel_efficiency removed - already included in rated capacity)
	prod.EstimatedOutputKW = s.config.RatedCapacityKW *
		ghiFactor *
		s.config.InverterEfficiency *
		tempAdjustment

	// Ensure non-negative
	if prod.EstimatedOutputKW < 0 {
		prod.EstimatedOutputKW = 0
	}

	// Calculate percentage of rated capacity
	prod.OutputPercentage = (prod.EstimatedOutputKW / s.config.RatedCapacityKW) * 100.0

	// Clamp to 0-100%
	if prod.OutputPercentage < 0 {
		prod.OutputPercentage = 0
	}
	if prod.OutputPercentage > 100 {
		prod.OutputPercentage = 100
	}

	return prod
}

// generateRecommendation generates actionable recommendation text
func (s *SolarForecastService) generateRecommendation(analysis *AlertAnalysis) string {
	if !analysis.CriteriaTriggered.AnyTriggered {
		return "Solar production forecast looks normal. No action required."
	}

	if analysis.CriteriaTriggered.LowProductionDurationTriggered {
		timeWindow := analysis.FirstLowProductionHour.Format("15:04") + "-" + analysis.LastLowProductionHour.Format("15:04")
		recommendation := fmt.Sprintf(
			"⚠️ Solar production will drop below %.1f kW for %d consecutive daylight hours during %s. "+
				"Expect severely limited power output during this period. "+
				"Consider reducing consumption or activating backup power sources. "+
				"Analysis uses automatic daylight detection based on solar irradiance.",
			s.config.ProductionAlertThresholdKW,
			analysis.ConsecutiveHourCount,
			timeWindow,
		)
		return recommendation
	}

	return "Solar production forecast looks normal. No action required."
}

// shouldSendAlert checks if alert should be sent based on current state
func (s *SolarForecastService) shouldSendAlert(ctx context.Context) (bool, error) {
	// Check if currently in daytime window
	now := time.Now()
	currentHour := now.Hour()

	if currentHour < s.config.DaytimeStartHour || currentHour >= s.config.DaytimeEndHour {
		s.logger.Debug("Outside daytime hours, skipping alert", "hour", currentHour, "start", s.config.DaytimeStartHour, "end", s.config.DaytimeEndHour)
		return false, nil
	}

	// Check if alert was already sent today
	state, err := s.stateRepository.GetLastAlertDate(ctx)
	if err != nil {
		return false, err
	}

	now = time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	s.logger.Info("Alert deduplication check", "alert_sent", state.AlertSent, "last_alert_date", state.LastAlertDate.Format("2006-01-02"), "today", today.Format("2006-01-02"))

	if state.AlertSent {
		s.logger.Info("AlertSent is true, checking date", "is_zero", state.LastAlertDate.IsZero())
		if !state.LastAlertDate.IsZero() {
			lastAlertDate := time.Date(state.LastAlertDate.Year(), state.LastAlertDate.Month(), state.LastAlertDate.Day(), 0, 0, 0, 0, state.LastAlertDate.Location())
			s.logger.Info("Comparing dates", "last_alert", lastAlertDate.Format("2006-01-02"), "today_compare", today.Format("2006-01-02"), "equal", lastAlertDate.Equal(today))
			if lastAlertDate.Equal(today) {
				s.logger.Info("Alert already sent today, skipping")
				return false, nil
			}
		}
	}

	return true, nil
}
