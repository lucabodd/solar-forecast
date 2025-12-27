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
	pushNotifier        PushNotifier
	stateRepository     AlertStateRepository
	logger              Logger
}

// NewSolarForecastService creates a new service instance
func NewSolarForecastService(
	config *Config,
	weatherProvider WeatherForecastProvider,
	emailNotifier EmailNotifier,
	pushNotifier PushNotifier,
	stateRepository AlertStateRepository,
	logger Logger,
) *SolarForecastService {
	return &SolarForecastService{
		config:          config,
		weatherProvider: weatherProvider,
		emailNotifier:   emailNotifier,
		pushNotifier:    pushNotifier,
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

	// Send push notification with chart if configured
	if s.pushNotifier != nil {
		title := "⚠️ Solar Production Alert"
		message := fmt.Sprintf("Low production: %d hours below %.1f kW\n%s-%s",
			analysis.ConsecutiveHourCount,
			s.config.ProductionAlertThresholdKW,
			analysis.FirstLowProductionHour.Format("15:04"),
			analysis.LastLowProductionHour.Format("15:04"))

		if analysis.HasRecovery {
			message += fmt.Sprintf("\n\nRecovery expected at %s (%d hours)",
				analysis.RecoveryHour.Format("15:04"),
				analysis.HoursUntilRecovery)
		}

		// Generate chart image if adapter supports it
		var chartImage []byte
		if chartGenerator, ok := s.pushNotifier.(interface {
			GenerateChartImage([]SolarProduction) ([]byte, error)
		}); ok {
			var err error
			chartImage, err = chartGenerator.GenerateChartImage(analysis.AllProductionHours)
			if err != nil {
				s.logger.Warn("Failed to generate chart image for push notification", "error", err.Error())
				// Continue without image
			} else {
				s.logger.Info("Generated chart image for push notification", "size_bytes", len(chartImage))
			}
		}

		if err := s.pushNotifier.SendNotification(ctx, title, message, chartImage); err != nil {
			s.logger.Warn("Failed to send push notification", "error", err.Error())
			// Don't fail the whole operation if push fails
		}
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

	// Calculate production for ALL hours (for chart display - shows night hours too)
	allProductionData := make([]SolarProduction, len(forecast.Hours))
	for i, hour := range forecast.Hours {
		allProductionData[i] = s.calculateSolarProduction(hour)
	}
	analysis.AllProductionHours = allProductionData

	// Filter hours to daylight analysis window
	windowHours := s.filterAnalysisWindow(forecast.Hours)
	if len(windowHours) == 0 {
		s.logger.Warn("No hours in analysis window")
		analysis.RecommendedAction = "No data in analysis window. Check again during daytime hours."
		return analysis
	}

	// Calculate solar production for daylight hours only (for alert analysis)
	productionData := make([]SolarProduction, len(windowHours))
	for i, hour := range windowHours {
		productionData[i] = s.calculateSolarProduction(hour)
	}

	// Evaluate low production duration criterion (using daylight hours only)
	s.evaluateLowProductionDuration(productionData, analysis)

	// Determine if any criterion triggered
	analysis.CriteriaTriggered.AnyTriggered = analysis.CriteriaTriggered.LowProductionDurationTriggered

	if !analysis.CriteriaTriggered.AnyTriggered {
		analysis.RecommendedAction = "Solar production forecast looks normal. No action required."
		return analysis
	}

	// If alert triggered but no recovery found in initial window,
	// search for recovery in the full 7-day forecast
	if !analysis.HasRecovery && analysis.CriteriaTriggered.LowProductionDurationTriggered {
		// Get ALL daylight hours from the full 7-day forecast for recovery search
		allDaylightHours := s.filterAnalysisWindow(forecast.Hours)
		allDaylightProduction := make([]SolarProduction, len(allDaylightHours))
		for i, hour := range allDaylightHours {
			allDaylightProduction[i] = s.calculateSolarProduction(hour)
		}
		s.findRecoveryInExtendedForecast(allDaylightProduction, analysis)
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

	for _, hour := range hours {
		// Include hour if there's meaningful solar radiation
		if hour.GlobalHorizontalIrradiance >= s.config.DaylightGHIThreshold {
			filtered = append(filtered, hour)
		}
	}

	s.logger.Debug("Filtered to daylight hours",
		"total_hours", len(hours),
		"daylight_hours", len(filtered),
		"ghi_threshold", s.config.DaylightGHIThreshold)

	return filtered
}

// evaluateLowProductionDuration checks if production drops below threshold for 6+ consecutive hours
func (s *SolarForecastService) evaluateLowProductionDuration(production []SolarProduction, analysis *AlertAnalysis) {
	var maxConsecutiveCount int
	var maxConsecutiveStart, maxConsecutiveEnd time.Time
	var maxConsecutiveHours []SolarProduction
	var recoveryHour time.Time
	var maxStreakRecovered bool

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
				maxStreakRecovered = false // Reset recovery flag for new max
			}
		} else {
			// Above threshold - RECOVERY DETECTED

			// If we just ended the max consecutive streak, capture recovery
			// Only count as recovery if GHI threshold is met (still daylight, not sunset)
			if currentConsecutiveCount > 0 && currentConsecutiveCount == maxConsecutiveCount && prod.GHI >= s.config.DaylightGHIThreshold {
				recoveryHour = prod.Hour
				maxStreakRecovered = true
				s.logger.Debug("Recovery detected",
					"recovery_hour", recoveryHour.Format("15:04"),
					"after_streak", maxConsecutiveCount,
					"ghi", prod.GHI,
				)
			}

			// Reset consecutive count
			currentConsecutiveCount = 0
			currentConsecutiveHours = nil
		}
	}

	s.logger.Debug("Low production duration evaluation",
		"threshold_kw", s.config.ProductionAlertThresholdKW,
		"duration_threshold_hours", s.config.DurationThresholdHours,
		"max_consecutive_hours", maxConsecutiveCount,
		"triggered", maxConsecutiveCount >= s.config.DurationThresholdHours,
		"has_recovery", maxStreakRecovered,
	)

	if maxConsecutiveCount >= s.config.DurationThresholdHours {
		analysis.CriteriaTriggered.LowProductionDurationTriggered = true
		analysis.ConsecutiveHourCount = maxConsecutiveCount
		analysis.FirstLowProductionHour = maxConsecutiveStart
		analysis.LastLowProductionHour = maxConsecutiveEnd
		analysis.LowProductionHours = maxConsecutiveHours

		// Set recovery fields
		analysis.HasRecovery = maxStreakRecovered
		if maxStreakRecovered {
			analysis.RecoveryHour = recoveryHour
			analysis.HoursUntilRecovery = int(recoveryHour.Sub(maxConsecutiveStart).Hours())
		}
	}
}

// findRecoveryInExtendedForecast searches for recovery in the full 7-day forecast
func (s *SolarForecastService) findRecoveryInExtendedForecast(allDaylightProduction []SolarProduction, analysis *AlertAnalysis) {
	// Look for the first daylight hour after LastLowProductionHour where production is above threshold
	for _, prod := range allDaylightProduction {
		// Only consider hours after the end of the low production period
		if prod.Hour.After(analysis.LastLowProductionHour) {
			// Check if production is above threshold and it's daylight
			if prod.EstimatedOutputKW >= s.config.ProductionAlertThresholdKW && prod.GHI >= s.config.DaylightGHIThreshold {
				analysis.HasRecovery = true
				analysis.RecoveryHour = prod.Hour
				analysis.HoursUntilRecovery = int(prod.Hour.Sub(analysis.FirstLowProductionHour).Hours())
				s.logger.Debug("Recovery found in extended forecast",
					"recovery_hour", analysis.RecoveryHour.Format("2006-01-02 15:04"),
					"hours_until_recovery", analysis.HoursUntilRecovery,
				)
				return
			}
		}
	}

	s.logger.Debug("No recovery found in 7-day forecast")
}

// calculateSolarProduction estimates solar output for a given hour
func (s *SolarForecastService) calculateSolarProduction(hour ForecastHour) SolarProduction {
	prod := SolarProduction{
		Hour:                     hour.Hour,
		CloudCover:               hour.CloudCover,
		Temperature:              hour.Temperature,
		GHI:                      hour.GlobalHorizontalIrradiance,
		PrecipitationProbability: hour.PrecipitationProbability,
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

	// Normalize GHI to reference (STC = 1000 W/m²)
	ghiFactor := hour.GlobalHorizontalIrradiance / STCIrradiance

	// Temperature adjustment (efficiency decreases with temperature above STC reference of 25°C)
	// TempCoefficient is typically -0.4 to -0.5 (%/°C)
	// At 45°C (20° above ref): 1.0 + (-0.4/100 * 20) = 0.92 (8% loss) ✓
	// At 5°C (20° below ref): 1.0 + (-0.4/100 * -20) = 1.08 (8% gain) ✓
	tempAdjustment := 1.0 + (s.config.TempCoefficient / 100.0 * (hour.Temperature - STCTemperature))

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
// DEPRECATED: This field is no longer displayed in alert emails as of the template update.
// Kept for backward compatibility and potential logging use.
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
	// Check if currently in daytime window (skip check in test mode)
	now := time.Now()
	if !s.config.TestMode {
		currentHour := now.Hour()

		if currentHour < s.config.DaytimeStartHour || currentHour >= s.config.DaytimeEndHour {
			s.logger.Debug("Outside daytime hours, skipping alert", "hour", currentHour, "start", s.config.DaytimeStartHour, "end", s.config.DaytimeEndHour)
			return false, nil
		}
	}

	// Check if alert was already sent today
	state, err := s.stateRepository.GetLastAlertDate(ctx)
	if err != nil {
		return false, err
	}

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
