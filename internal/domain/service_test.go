package domain

import (
	"testing"
	"time"
)

// mockLogger for testing
type mockLogger struct{}

func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}

func TestCalculateSolarProduction(t *testing.T) {
	config := &Config{
		RatedCapacityKW:    8.9,
		InverterEfficiency: 0.97,
		TempCoefficient:    -0.4,
	}

	service := &SolarForecastService{
		config: config,
		logger: &mockLogger{},
	}

	tests := []struct {
		name           string
		hour           ForecastHour
		wantMinKW      float64
		wantMaxKW      float64
		wantPercentMin float64
		wantPercentMax float64
	}{
		{
			name: "full sun at STC (1000 W/m², 25°C)",
			hour: ForecastHour{
				Hour:                       time.Now(),
				GlobalHorizontalIrradiance: 1000.0,
				Temperature:                25.0,
				CloudCover:                 0,
			},
			wantMinKW:      8.5, // 8.9 * 0.97 = 8.633
			wantMaxKW:      8.7,
			wantPercentMin: 95,
			wantPercentMax: 100,
		},
		{
			name: "partial sun (600 W/m², 25°C)",
			hour: ForecastHour{
				Hour:                       time.Now(),
				GlobalHorizontalIrradiance: 600.0,
				Temperature:                25.0,
				CloudCover:                 50,
			},
			wantMinKW:      5.0, // 8.9 * 0.6 * 0.97 = 5.18
			wantMaxKW:      5.3,
			wantPercentMin: 55,
			wantPercentMax: 60,
		},
		{
			name: "hot day (1000 W/m², 45°C) - should lose ~8%",
			hour: ForecastHour{
				Hour:                       time.Now(),
				GlobalHorizontalIrradiance: 1000.0,
				Temperature:                45.0, // 20°C above reference
				CloudCover:                 0,
			},
			wantMinKW:      7.8, // 8.9 * 0.97 * 0.92 = 7.94
			wantMaxKW:      8.1,
			wantPercentMin: 87,
			wantPercentMax: 92,
		},
		{
			name: "cold day (1000 W/m², 5°C) - should gain ~8%",
			hour: ForecastHour{
				Hour:                       time.Now(),
				GlobalHorizontalIrradiance: 1000.0,
				Temperature:                5.0, // 20°C below reference
				CloudCover:                 0,
			},
			wantMinKW:      9.2, // 8.9 * 0.97 * 1.08 = 9.32 (but clamped to 100%)
			wantMaxKW:      9.5,
			wantPercentMin: 100, // Clamped to 100%
			wantPercentMax: 100,
		},
		{
			name: "nighttime (0 W/m²)",
			hour: ForecastHour{
				Hour:                       time.Now(),
				GlobalHorizontalIrradiance: 0.0,
				Temperature:                15.0,
				CloudCover:                 100,
			},
			wantMinKW:      0,
			wantMaxKW:      0.01,
			wantPercentMin: 0,
			wantPercentMax: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateSolarProduction(tt.hour)

			if result.EstimatedOutputKW < tt.wantMinKW || result.EstimatedOutputKW > tt.wantMaxKW {
				t.Errorf("EstimatedOutputKW = %.2f, want between %.2f and %.2f",
					result.EstimatedOutputKW, tt.wantMinKW, tt.wantMaxKW)
			}

			if result.OutputPercentage < tt.wantPercentMin || result.OutputPercentage > tt.wantPercentMax {
				t.Errorf("OutputPercentage = %.1f%%, want between %.1f%% and %.1f%%",
					result.OutputPercentage, tt.wantPercentMin, tt.wantPercentMax)
			}
		})
	}
}

func TestTemperatureAdjustment(t *testing.T) {
	// This test specifically verifies the temperature coefficient calculation
	// is working correctly (the bug that was fixed)
	config := &Config{
		RatedCapacityKW:    10.0, // Easy math
		InverterEfficiency: 1.0,  // No inverter loss for simple calculation
		TempCoefficient:    -0.4, // -0.4%/°C
	}

	service := &SolarForecastService{
		config: config,
		logger: &mockLogger{},
	}

	tests := []struct {
		name       string
		temp       float64
		wantFactor float64 // Expected temperature adjustment factor
	}{
		{
			name:       "at reference (25°C) - no adjustment",
			temp:       25.0,
			wantFactor: 1.0,
		},
		{
			name:       "10°C above reference (35°C) - 4% loss",
			temp:       35.0,
			wantFactor: 0.96, // 1 + (-0.4/100 * 10) = 0.96
		},
		{
			name:       "20°C above reference (45°C) - 8% loss",
			temp:       45.0,
			wantFactor: 0.92, // 1 + (-0.4/100 * 20) = 0.92
		},
		{
			name:       "10°C below reference (15°C) - 4% gain",
			temp:       15.0,
			wantFactor: 1.04, // 1 + (-0.4/100 * -10) = 1.04
		},
		{
			name:       "20°C below reference (5°C) - 8% gain",
			temp:       5.0,
			wantFactor: 1.08, // 1 + (-0.4/100 * -20) = 1.08
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour := ForecastHour{
				Hour:                       time.Now(),
				GlobalHorizontalIrradiance: STCIrradiance, // Full sun
				Temperature:                tt.temp,
			}

			result := service.calculateSolarProduction(hour)

			// With 10kW capacity, 100% inverter efficiency, and full sun,
			// the output should equal the temperature adjustment factor * 10
			expectedKW := tt.wantFactor * 10.0
			tolerance := 0.01

			if result.EstimatedOutputKW < expectedKW-tolerance || result.EstimatedOutputKW > expectedKW+tolerance {
				t.Errorf("At %.0f°C: EstimatedOutputKW = %.3f, want %.3f (factor = %.2f)",
					tt.temp, result.EstimatedOutputKW, expectedKW, tt.wantFactor)
			}
		})
	}
}

func TestEvaluateLowProductionDuration(t *testing.T) {
	config := &Config{
		ProductionAlertThresholdKW: 2.0,
		DurationThresholdHours:     3, // Alert after 3 consecutive hours
		DaylightGHIThreshold:       50.0,
	}

	service := &SolarForecastService{
		config: config,
		logger: &mockLogger{},
	}

	baseTime := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		production        []SolarProduction
		wantTriggered     bool
		wantConsecutive   int
		wantHasRecovery   bool
	}{
		{
			name: "no low production - all above threshold",
			production: []SolarProduction{
				{Hour: baseTime, EstimatedOutputKW: 5.0, GHI: 800},
				{Hour: baseTime.Add(1 * time.Hour), EstimatedOutputKW: 4.5, GHI: 750},
				{Hour: baseTime.Add(2 * time.Hour), EstimatedOutputKW: 4.0, GHI: 700},
			},
			wantTriggered:   false,
			wantConsecutive: 0,
		},
		{
			name: "2 hours low - below threshold count",
			production: []SolarProduction{
				{Hour: baseTime, EstimatedOutputKW: 1.0, GHI: 200},
				{Hour: baseTime.Add(1 * time.Hour), EstimatedOutputKW: 1.5, GHI: 250},
				{Hour: baseTime.Add(2 * time.Hour), EstimatedOutputKW: 4.0, GHI: 700},
			},
			wantTriggered:   false,
			wantConsecutive: 0,
		},
		{
			name: "3 consecutive hours low - triggers alert",
			production: []SolarProduction{
				{Hour: baseTime, EstimatedOutputKW: 1.0, GHI: 200},
				{Hour: baseTime.Add(1 * time.Hour), EstimatedOutputKW: 1.5, GHI: 250},
				{Hour: baseTime.Add(2 * time.Hour), EstimatedOutputKW: 1.2, GHI: 220},
				{Hour: baseTime.Add(3 * time.Hour), EstimatedOutputKW: 4.0, GHI: 700},
			},
			wantTriggered:     true,
			wantConsecutive:   3,
			wantHasRecovery:   true,
		},
		{
			name: "5 consecutive hours low with recovery",
			production: []SolarProduction{
				{Hour: baseTime, EstimatedOutputKW: 1.0, GHI: 200},
				{Hour: baseTime.Add(1 * time.Hour), EstimatedOutputKW: 1.5, GHI: 250},
				{Hour: baseTime.Add(2 * time.Hour), EstimatedOutputKW: 1.2, GHI: 220},
				{Hour: baseTime.Add(3 * time.Hour), EstimatedOutputKW: 0.8, GHI: 180},
				{Hour: baseTime.Add(4 * time.Hour), EstimatedOutputKW: 1.1, GHI: 200},
				{Hour: baseTime.Add(5 * time.Hour), EstimatedOutputKW: 5.0, GHI: 900},
			},
			wantTriggered:     true,
			wantConsecutive:   5,
			wantHasRecovery:   true,
		},
		{
			name: "interrupted low production - takes longest streak",
			production: []SolarProduction{
				{Hour: baseTime, EstimatedOutputKW: 1.0, GHI: 200},
				{Hour: baseTime.Add(1 * time.Hour), EstimatedOutputKW: 1.5, GHI: 250},
				{Hour: baseTime.Add(2 * time.Hour), EstimatedOutputKW: 3.0, GHI: 500}, // Above threshold
				{Hour: baseTime.Add(3 * time.Hour), EstimatedOutputKW: 0.8, GHI: 180},
				{Hour: baseTime.Add(4 * time.Hour), EstimatedOutputKW: 1.1, GHI: 200},
				{Hour: baseTime.Add(5 * time.Hour), EstimatedOutputKW: 1.2, GHI: 210},
				{Hour: baseTime.Add(6 * time.Hour), EstimatedOutputKW: 0.9, GHI: 190},
			},
			wantTriggered:     true,
			wantConsecutive:   4, // Second streak is longer
			wantHasRecovery:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &AlertAnalysis{}
			service.evaluateLowProductionDuration(tt.production, analysis)

			if analysis.CriteriaTriggered.LowProductionDurationTriggered != tt.wantTriggered {
				t.Errorf("Triggered = %v, want %v",
					analysis.CriteriaTriggered.LowProductionDurationTriggered, tt.wantTriggered)
			}

			if tt.wantTriggered && analysis.ConsecutiveHourCount != tt.wantConsecutive {
				t.Errorf("ConsecutiveHourCount = %d, want %d",
					analysis.ConsecutiveHourCount, tt.wantConsecutive)
			}

			if analysis.HasRecovery != tt.wantHasRecovery {
				t.Errorf("HasRecovery = %v, want %v",
					analysis.HasRecovery, tt.wantHasRecovery)
			}
		})
	}
}

func TestFilterAnalysisWindow(t *testing.T) {
	config := &Config{
		DaylightGHIThreshold: 50.0,
	}

	service := &SolarForecastService{
		config: config,
		logger: &mockLogger{},
	}

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	hours := []ForecastHour{
		{Hour: baseTime.Add(0 * time.Hour), GlobalHorizontalIrradiance: 0},   // Night
		{Hour: baseTime.Add(6 * time.Hour), GlobalHorizontalIrradiance: 30},  // Dawn, below threshold
		{Hour: baseTime.Add(7 * time.Hour), GlobalHorizontalIrradiance: 100}, // Day
		{Hour: baseTime.Add(12 * time.Hour), GlobalHorizontalIrradiance: 800}, // Midday
		{Hour: baseTime.Add(17 * time.Hour), GlobalHorizontalIrradiance: 60},  // Dusk
		{Hour: baseTime.Add(18 * time.Hour), GlobalHorizontalIrradiance: 40},  // Below threshold
		{Hour: baseTime.Add(20 * time.Hour), GlobalHorizontalIrradiance: 0},   // Night
	}

	filtered := service.filterAnalysisWindow(hours)

	if len(filtered) != 3 {
		t.Errorf("len(filtered) = %d, want 3 (hours with GHI >= 50)", len(filtered))
	}

	// Verify only daylight hours are included
	for _, h := range filtered {
		if h.GlobalHorizontalIrradiance < 50 {
			t.Errorf("Hour %v has GHI %.0f, should not be included",
				h.Hour.Format("15:04"), h.GlobalHorizontalIrradiance)
		}
	}
}
