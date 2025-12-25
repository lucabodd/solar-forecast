# Solar Forecast Warning System

A Go microservice that monitors 48-hour solar production forecasts and sends **email + push notifications** when adverse weather is expected to reduce solar output significantly.

## Features

- ⚡ **Real-time Solar Forecast Monitoring** - Fetches 48-hour forecasts from Open-Meteo API
- 📧 **HTML Email Alerts** - Beautiful responsive emails with cumulative kWh charts and color-coded weather tables
- 📱 **Pushover Push Notifications** - Instant mobile alerts with production details
- 🎯 **Smart Alert Criteria** - Duration-based alerts (e.g., production < 2kW for 6+ consecutive hours)
- 🌅 **Automatic Daylight Detection** - Uses GHI (solar irradiance) instead of fixed time windows
- 🔄 **Recovery Notifications** - Automatic emails when conditions improve
- 🔒 **Secure Configuration** - Environment variable support for credentials
- 🧪 **Testing Utilities** - `make mail` command for easy testing
- 📱 **Mobile Responsive** - Email templates optimized for mobile devices
- 🏗️ **Hexagonal Architecture** - Clean separation of concerns, easy to test and extend

## Quick Start

See [QUICKSTART.md](QUICKSTART.md) for step-by-step setup instructions.

## Installation

See [INSTALLATION.md](INSTALLATION.md) for detailed installation guide.

## Configuration

### Step 1: Copy Template

```bash
cp config/application.properties.template config/application.properties
```

### Step 2: Configure Your Settings

Edit `config/application.properties`:

```properties
# Your Location
latitude=39.47
longitude=-0.38

# Alert Thresholds
production_alert_threshold_kw=2.0    # Alert if below 2.0 kW
duration_threshold_hours=6           # Alert if below threshold for 6+ hours

# Solar Panel Configuration
rated_capacity_kw=8.9                # Your system capacity (16×560W)
inverter_efficiency=0.97             # DC to AC efficiency
temp_coefficient=-0.4                # Temperature derating

# Email Credentials (required)
gmail_sender=your-email@gmail.com
gmail_app_password=YOUR_APP_PASSWORD_HERE
recipient_email=your-email@gmail.com

# Pushover Push Notifications (optional)
pushover_user_key=YOUR_PUSHOVER_USER_KEY
pushover_api_token=YOUR_PUSHOVER_API_TOKEN
```

### Step 3: Environment Variables (Optional)

You can override sensitive values using environment variables:

```bash
export SOLAR_GMAIL_APP_PASSWORD="your-app-password"
export SOLAR_PUSHOVER_USER_KEY="your-user-key"
export SOLAR_PUSHOVER_API_TOKEN="your-api-token"
```

## Gmail Setup

1. Go to [Google Account Security](https://myaccount.google.com/security)
2. Enable "2-Step Verification"
3. Go to [App Passwords](https://myaccount.google.com/apppasswords)
4. Create app password for "Mail"
5. Copy the generated password to your config file

## Pushover Setup (Optional)

1. Sign up at [pushover.net](https://pushover.net)
2. Install Pushover app on your mobile device
3. Create an application at [pushover.net/apps/build](https://pushover.net/apps/build)
4. Copy your **User Key** and **API Token** to config file

## Usage

### Run Once

```bash
make run
```

### Run with Debug Logging

```bash
make run-debug
```

### Test Email/Push Alerts

```bash
make mail
```

This command:
- Clears alert state (allows re-sending)
- Lowers thresholds to 5.0 kW and 1 hour (triggers easily)
- Runs the application in test mode
- Sends both email and push notifications if configured

### Set Up Cron (Hourly Monitoring)

```bash
make install        # Install to /usr/local/bin
make cron-install   # Interactive cron setup
```

Example cron job (runs hourly from 6am-6pm):
```
0 6-18 * * * /usr/local/bin/solar-forecast -config /path/to/config.properties >> /var/log/solar-forecast.log 2>&1
```

## Architecture

This application follows **hexagonal architecture** (ports & adapters pattern):

```
cmd/
└── solar-forecast/
    └── main.go                     # Application entry point

internal/
├── domain/
│   ├── models.go                  # Core domain models and interfaces
│   └── service.go                 # Business logic (SolarForecastService)
├── adapters/
│   ├── openmeteo.go               # Weather API integration
│   ├── gmail.go                   # Email notifications
│   ├── pushover.go                # Push notifications
│   ├── filestate.go               # Alert state persistence
│   └── logger.go                  # Logging implementation
└── config/
    └── loader.go                  # Configuration management

config/
├── application.properties.template # Configuration template (committed)
└── application.properties          # User config (git-ignored)
```

### Key Design Principles

- **Ports & Adapters**: Domain logic is independent of external services
- **Dependency Injection**: All dependencies injected through interfaces
- **Single Responsibility**: Each adapter has one job
- **Testability**: Domain logic can be tested without external dependencies

## How It Works

### 1. Fetch Forecast
- Retrieves 48-hour weather data from Open-Meteo API
- Includes temperature, cloud cover, GHI (solar irradiance), humidity

### 2. Calculate Production
- For each hour: `P = P_rated × (GHI/1000) × η_inverter × temp_adjustment`
- Temperature derating: `1 - (temp_coefficient/100 × (temp - 25))`
- Automatic daylight filtering using GHI threshold (50 W/m²)

### 3. Analyze Criteria
- Identifies consecutive hours below production threshold
- Detects recovery point (when production rises above threshold)
- Calculates duration and time windows

### 4. Send Alerts
- **Email**: HTML formatted with:
  - Cumulative kWh production chart (next 12 hours)
  - Color-coded hourly weather table (green=good, red=low production)
  - Recovery forecast section
  - Responsive design for mobile devices
- **Push Notification**: Text summary with:
  - Duration and time window
  - Recovery time (if detected)
  - Sent to Pushover app on mobile device

### 5. Track State
- Prevents duplicate alerts (one per day maximum)
- Sends recovery email when conditions improve
- State persisted to `~/.solar-forecast/alert_state.json`

## Alert Logic

**Alert triggers when:**
- Production < threshold (default: 2.0 kW)
- For duration >= threshold (default: 6 consecutive hours)
- During daylight hours (GHI >= 50 W/m²)
- Alert not already sent today

**Example:**
```
Config: 2.0 kW threshold, 6 hours duration
Forecast:
  14:00 - 1.8 kW ❌
  15:00 - 1.5 kW ❌
  16:00 - 1.7 kW ❌
  17:00 - 1.6 kW ❌
  18:00 - 1.9 kW ❌
  19:00 - 1.8 kW ❌ (6 hours total)
  20:00 - 2.3 kW ✓ (recovery detected)

Result: Alert sent at 19:00 showing low production from 14:00-19:00
Recovery expected at 20:00 (6 hours until recovery)
```

## Email Preview

The alert email includes:
- **Alert Banner**: Specific details about threshold and duration
- **Metrics Cards**:
  - Production < 2kW: X HOURS (or ✓ OK)
  - Low Production Period: HH:MM-HH:MM
- **Cumulative kWh Chart**: Shows total energy production over next 12 hours
- **12-Hour Weather Table**: Color-coded rows (green=good, red=low production)
- **Recovery Forecast**: When conditions will improve

All sections are **responsive** and display properly on mobile devices.

## Testing

### Test Email Alert

```bash
make mail
```

This runs the application in test mode with:
- Production threshold: 5.0 kW (higher, triggers more easily)
- Duration threshold: 1 hour (shorter, triggers quickly)
- Alert state cleared (allows immediate re-sending)

### Manual Testing

```bash
# Run with environment variable override
SOLAR_TEST_MODE=1 ./solar-forecast -config config/application.properties -debug
```

## Troubleshooting

### Alert Not Sending

1. **Check credentials**:
   ```bash
   # Verify config file
   grep gmail_app_password config/application.properties
   ```

2. **Check forecast conditions**:
   ```bash
   # Run with debug logging
   make run-debug
   ```

3. **Verify daytime hours**:
   - Alerts only sent during daytime (configured hours)
   - Default: 8am-8pm

### Push Notifications Not Working

1. **Verify Pushover config**:
   - User key and API token must be set
   - Cannot be placeholder values

2. **Check Pushover app**:
   - App installed on mobile device
   - Logged in with correct account

3. **Test manually**:
   ```bash
   curl -X POST https://api.pushover.net/1/messages.json \
     -d "token=YOUR_API_TOKEN" \
     -d "user=YOUR_USER_KEY" \
     -d "message=Test"
   ```

### Email Not Responsive on Mobile

- This should be fixed in the latest version
- Email includes viewport meta tag and media queries
- Test on multiple email clients (Gmail, Apple Mail, Outlook)

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guide (to be created).

## Security Notes

- **NEVER commit `config/application.properties`** - it contains credentials
- Use environment variables for production deployments
- The `.gitignore` file protects sensitive config files
- Template file (`application.properties.template`) is safe to commit

## License

MIT License - see LICENSE file for details

## Credits

- Weather data provided by [Open-Meteo API](https://open-meteo.com)
- Push notifications via [Pushover](https://pushover.net)
- Built with Go 1.19+

## Support

For issues or questions:
1. Check existing [GitHub Issues](https://github.com/your-repo/issues)
2. Create a new issue with:
   - Go version (`go version`)
   - OS and architecture
   - Config file (redacted credentials)
   - Log output with `-debug` flag
