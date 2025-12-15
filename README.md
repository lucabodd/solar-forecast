# Solar Forecast Warning System

A Go microservice that monitors solar production forecasts and sends email alerts when adverse weather is expected to reduce solar output significantly.

## Architecture

This application follows a **hexagonal microservice architecture** (ports & adapters pattern):

```
cmd/
├── solar-forecast/
│   └── main.go                 # CLI entry point
│
internal/
├── domain/
│   ├── models.go              # Core domain models
│   └── service.go             # Business logic (SolarForecastService)
├── ports/
│   └── ports.go               # Port interfaces (abstractions)
├── adapters/
│   ├── openmeteo.go           # Weather API adapter
│   ├── gmail.go               # Email notification adapter
│   ├── filestate.go           # File-based alert state storage
│   └── logger.go              # Logging adapter
└── config/
    └── loader.go              # Configuration file parser

config/
└── application.properties     # Configuration file (user-editable)
```

## Features

- ✅ **Real-time Solar Forecast Monitoring** - Fetches 48-hour forecasts from Open-Meteo API
- ✅ **Three Alert Criteria** - Triggers on ANY of: high cloud cover, low irradiance (GHI), or low output percentage
- ✅ **Majority Logic** - Alert only if majority of hours in analysis window meet criteria
- ✅ **HTML Email Alerts** - Beautiful formatted emails with charts and graphs
- ✅ **Once-Per-Day Alerts** - Prevents alert spam; resets at midnight
- ✅ **Configurable Analysis Window** - Set specific hours to monitor (e.g., 10am-4pm peak hours)
- ✅ **Automatic Retry Logic** - Configurable retries for API calls with exponential backoff
- ✅ **Hexagonal Architecture** - Clean separation of concerns, easy to test and extend

## Installation

### Prerequisites

- Go 1.19+
- Gmail account with app password enabled

### Setup Gmail App Password

1. Go to [Google Account Security](https://myaccount.google.com/security)
2. Enable "2-Step Verification" if not already enabled
3. Go to App Passwords section
4. Select "Mail" and "macOS" (or your platform)
5. Generate app password (16 characters, with spaces)
6. Copy the password - you'll need this for configuration

### Build the Application

```bash
# Clone/navigate to the project
cd /Users/b0d/Workspace/repos/solar-forecast

# Download dependencies
go mod tidy

# Build the binary
go build -o solar-forecast ./cmd/solar-forecast
```

### Configuration

Edit `config/application.properties`:

```properties
# Your location
latitude=52.52
longitude=13.41

# Alert thresholds (trigger if ANY are met)
cloud_cover_threshold=80              # Alert if ≥80% clouds
ghi_threshold=200                     # Alert if ≤200 W/m² irradiance
output_percentage_threshold=30        # Alert if ≤30% of capacity

# Analysis window (hours to check, 24-hour format)
analysis_window_start=10              # Start at 10am
analysis_window_end=16                # End at 4pm

# Solar panel configuration
rated_capacity_kw=5.0                 # Panel capacity
panel_efficiency=0.20                 # 20% efficiency
inverter_efficiency=0.97              # 97% efficiency
temp_coefficient=-0.4                 # -0.4% per °C above 25°C

# Email configuration
gmail_sender=your-email@gmail.com
recipient_email=your-email@gmail.com
gmail_app_password=xxxx xxxx xxxx xxxx    # From step above

# Cron scheduling hours
daytime_start_hour=6                  # Start monitoring at 6am
daytime_end_hour=18                   # Stop at 6pm

# API retry configuration
api_retry_attempts=3                  # Retry 3 times on failure
api_retry_delay_seconds=5             # Wait 5 seconds between retries
api_timeout_seconds=10                # API call timeout
```

## Usage

### Run Manually

```bash
# Basic run
./solar-forecast -config config/application.properties

# With debug logging
./solar-forecast -config config/application.properties -debug

# Custom state directory
./solar-forecast -config config/application.properties -state ~/.solar-forecast
```

### Schedule with Cron

Edit your crontab:

```bash
crontab -e
```

Add these lines to run hourly during daytime (6am-6pm):

```cron
# Run Solar Forecast Check
# Hourly from 6am to 6pm every day
0 6-18 * * * /path/to/solar-forecast -config /path/to/config/application.properties

# Optional: With logging to file
0 6-18 * * * /path/to/solar-forecast -config /path/to/config/application.properties >> /var/log/solar-forecast.log 2>&1
```

**Example crontab setup (macOS/Linux):**

```bash
# Copy binary to standard location
sudo cp solar-forecast /usr/local/bin/

# Create log directory
mkdir -p ~/var/log

# Add to crontab
crontab -e
```

Then add:
```cron
0 6-18 * * * solar-forecast -config ~/Workspace/repos/solar-forecast/config/application.properties >> ~/var/log/solar-forecast.log 2>&1
```

## Alert Criteria Explanation

### 1. Cloud Cover Threshold
- **Default:** 80%
- **Meaning:** Alert if 80% or more of the sky is covered by clouds
- **Impact:** Clouds reduce solar irradiance significantly (10% cloud ≈ 15% power reduction)

### 2. Global Horizontal Irradiance (GHI) Threshold
- **Default:** 200 W/m²
- **Meaning:** Alert if solar radiation drops to 200 W/m² or below
- **Reference:** Clear day = ~1000 W/m² peak, cloudy day = 200-400 W/m²

### 3. Estimated Output Percentage
- **Default:** 30%
- **Meaning:** Alert if estimated solar output drops to 30% or below panel capacity
- **Formula:** `Output % = (GHI/1000) × Panel Efficiency × Inverter Efficiency × Temperature Adjustment × Cloud Factor`

## Alert Logic

An alert is **TRIGGERED** if:

1. **ANY** of the three criteria is met (cloud cover OR GHI OR output %)
2. **MAJORITY** of hours in the analysis window meet that criterion
3. Current time is within **daytime hours** (default 6am-6pm)
4. **No alert** was sent yet today (resets at midnight)

Example: If `analysis_window_start=10` and `analysis_window_end=16`:
- System checks 10am-4pm period
- If 4 out of 6 hours have >80% cloud cover → criterion triggered
- If today's alert hasn't been sent → email sent

## Email Output

The email includes:

- ☀️ **Header** with alert timestamp
- ⚠️ **Alert notification** explaining the issue
- 📊 **Metric cards** showing status of each criterion
- ☁️ **Cloud cover chart** (hourly breakdown)
- ⚡ **Solar output chart** (estimated % of capacity)
- 📋 **Detailed analysis** of affected hours
- 💡 **Actionable recommendation** based on conditions

## Architecture Benefits

### Hexagonal (Ports & Adapters)
- **Domain Layer**: Core business logic isolated from external dependencies
- **Ports**: Abstract interfaces for external services
- **Adapters**: Concrete implementations (Open-Meteo, Gmail, File Storage)

### Easy to Test
```go
// Mock any adapter in tests
mockWeatherProvider := &MockWeatherProvider{}
service := NewSolarForecastService(cfg, mockWeatherProvider, ...)
```

### Easy to Extend
- Add new weather API? Create a new adapter implementing `WeatherForecastProvider`
- Add Slack notifications? Create a new adapter implementing `EmailNotifier`
- Add database storage? Create a new adapter implementing `AlertStateRepository`

## Troubleshooting

### "Failed to send email"
- Verify Gmail app password is correct (copy full 16-character password with spaces)
- Ensure 2-Step Verification is enabled on your Gmail account
- Check that sender email matches the account that generated the app password

### "No alert criteria triggered"
- Check that your solar panels are actually at the configured location
- Verify thresholds are realistic for your region
- Run with `-debug` flag to see forecast values

### "Alert already sent today"
- State is tracked in `~/.solar-forecast/alert_state.json`
- To reset: `rm ~/.solar-forecast/alert_state.json`
- Resets automatically at midnight

### "API timeout or connection error"
- Check internet connection
- Verify you can reach `https://api.open-meteo.com/v1/forecast`
- Open-Meteo is free and no API key is required
- Retry logic will automatically retry up to 3 times

## Dependencies

The application uses only Go standard library with no external dependencies:

- `net/http` - HTTP client for API calls
- `net/smtp` - SMTP for email
- `encoding/json` - JSON parsing
- `os`, `io`, `bufio` - File operations
- `time`, `context` - Timing and cancellation

## Performance

- **API Response Time:** ~500ms to 2 seconds
- **Email Send Time:** ~2-5 seconds
- **Total Execution:** ~5-10 seconds per run
- **Memory Usage:** ~10MB
- **CPU Usage:** Minimal (I/O bound)

## Data Privacy

- Configuration file contains your Gmail app password (keep secure)
- State file stored locally at `~/.solar-forecast/alert_state.json`
- No data is sent anywhere except to Open-Meteo (public API) and Gmail SMTP
- All processing happens locally

## License

MIT License - See LICENSE file

## Support

For issues, feature requests, or contributions, please create an issue in the repository.

---

**Note:** This application is designed for personal use. Forecast accuracy is ±15-20% due to inherent limitations in weather prediction. Use as a guide, not as the sole basis for critical decisions.
