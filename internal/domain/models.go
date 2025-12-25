package domain

import (
	"context"
	"time"
)

// Logger defines the interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// WeatherForecastProvider defines the interface for fetching weather forecast data
type WeatherForecastProvider interface {
	// GetForecast retrieves 48-hour weather forecast for given coordinates
	GetForecast(ctx context.Context, latitude, longitude float64) (*ForecastData, error)
}

// EmailNotifier defines the interface for sending email notifications
type EmailNotifier interface {
	// SendAlert sends an HTML-formatted alert email with graphs
	SendAlert(ctx context.Context, analysis *AlertAnalysis) error
	// SendRecoveryEmail sends an email indicating conditions have improved
	SendRecoveryEmail(ctx context.Context) error
}

// PushNotifier defines the interface for sending push notifications
type PushNotifier interface {
	// SendNotification sends a push notification with optional image
	SendNotification(ctx context.Context, title, message string, imageData []byte) error
}

// AlertStateRepository defines the interface for persisting alert state
type AlertStateRepository interface {
	// GetLastAlertDate retrieves the last date when alert was sent
	GetLastAlertDate(ctx context.Context) (AlertState, error)

	// SaveAlertDate persists the current alert sent date
	SaveAlertDate(ctx context.Context, state AlertState) error

	// ResetIfNewDay resets alert state if it's a new calendar day (returns true if reset)
	ResetIfNewDay(ctx context.Context) (bool, error)

	// MarkAlertSent marks that alert was sent today
	MarkAlertSent(ctx context.Context) error

	// ShouldSendAlert checks if alert should be sent based on state
	ShouldSendAlert(ctx context.Context) (bool, error)

	// ShouldSendRecoveryEmail checks if recovery email should be sent
	ShouldSendRecoveryEmail(ctx context.Context) (bool, error)

	// MarkRecoveryEmailSent marks that recovery email has been sent
	MarkRecoveryEmailSent(ctx context.Context) error
}

// Config holds all application configuration
type Config struct {
	// Location
	Latitude  float64
	Longitude float64

	// Alert thresholds (duration-based)
	ProductionAlertThresholdKW float64 // Alert if production drops below this (kW)
	DurationThresholdHours     int     // Alert if threshold exceeded for this many consecutive hours

	// Analysis window
	AnalysisWindowStart int // hour 0-23
	AnalysisWindowEnd   int // hour 0-23

	// Panel configuration
	RatedCapacityKW    float64 // Rated output at STC (already includes panel efficiency)
	PanelEfficiency    float64 // DEPRECATED - not used (rated capacity already accounts for this)
	InverterEfficiency float64 // DC to AC conversion efficiency (0.95-0.98)
	TempCoefficient    float64 // Temperature coefficient in % per °C (typically -0.4 to -0.5)

	// Email
	GmailAppPassword string
	GmailSender      string
	RecipientEmail   string

	// Pushover push notifications
	PushoverUserKey   string
	PushoverAPIToken  string

	// Cron
	DaytimeStartHour int // hour 0-23
	DaytimeEndHour   int // hour 0-23

	// API retry
	APIRetryAttempts     int
	APIRetryDelaySeconds int
	APITimeoutSeconds    int
}

// ForecastHour represents one hour of forecast data
type ForecastHour struct {
	Hour                 time.Time
	Temperature          float64 // Celsius
	CloudCover           int     // percentage 0-100
	GlobalHorizontalIrradiance float64 // W/m²
	RelativeHumidity     int     // percentage 0-100
}

// ForecastData holds 48-hour forecast
type ForecastData struct {
	Hours []ForecastHour
}

// SolarProduction represents calculated solar production for an hour
type SolarProduction struct {
	Hour              time.Time
	EstimatedOutputKW float64
	OutputPercentage  float64 // percentage of rated capacity

	// Weather context for email rendering
	CloudCover  int     // percentage 0-100
	Temperature float64 // Celsius
	GHI         float64 // W/m² - for condition determination
}

// AlertCriteria represents which thresholds were triggered
type AlertCriteria struct {
	LowProductionDurationTriggered bool // Alert when production < threshold for 6+ consecutive hours
	AnyTriggered                   bool
}

// AlertAnalysis holds detailed analysis of forecast period
type AlertAnalysis struct {
	CriteriaTriggered      AlertCriteria
	LowProductionHours     []SolarProduction // Hours with production < threshold
	AllProductionHours     []SolarProduction // All forecast hours (for chart display)
	ConsecutiveHourCount   int               // How many consecutive hours triggered
	FirstLowProductionHour time.Time         // Start of low production period
	LastLowProductionHour  time.Time         // End of low production period
	RecommendedAction      string

	// Recovery tracking
	RecoveryHour       time.Time // When production rises above threshold
	HoursUntilRecovery int       // Total duration from start to recovery
	HasRecovery        bool      // Whether recovery happens within 48h forecast
}

// AlertState tracks whether alert was sent today
type AlertState struct {
	LastAlertDate     time.Time
	AlertSent         bool
	AlertRecovered    bool // Track if conditions improved
	RecoveryEmailSent bool // Flag to ensure recovery email only sent once
}
