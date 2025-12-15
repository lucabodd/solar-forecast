# Quick Start Guide

## 1. Configuration Setup

Edit `config/application.properties` with your settings:

### Required Settings

```properties
# Your location (find on Google Maps: right-click → coordinates)
latitude=52.52
longitude=13.41

# Gmail configuration
gmail_sender=your-email@gmail.com
recipient_email=your-email@gmail.com
gmail_app_password=xxxx xxxx xxxx xxxx    # From Gmail settings
```

### Optional Customization

```properties
# Alert thresholds (trigger if ANY are met)
cloud_cover_threshold=80              # 0-100%
ghi_threshold=200                     # W/m² (watts per square meter)
output_percentage_threshold=30        # % of rated capacity

# Analysis window (hours to check)
analysis_window_start=10              # 24-hour format (10am)
analysis_window_end=16                # 24-hour format (4pm)

# Solar panel specs
rated_capacity_kw=5.0                 # Your system's peak power
panel_efficiency=0.20                 # 15-22% typical
inverter_efficiency=0.97              # 95-98% typical
temp_coefficient=-0.4                 # -0.3 to -0.5 typical

# Cron execution window
daytime_start_hour=6                  # Start monitoring
daytime_end_hour=18                   # Stop monitoring

# API retry configuration
api_retry_attempts=3                  # Number of retries
api_retry_delay_seconds=5             # Wait time between retries
api_timeout_seconds=10                # API call timeout
```

## 2. Build

```bash
cd /Users/b0d/Workspace/repos/solar-forecast
make build
```

## 3. Test Run

```bash
# Test with your configuration
./solar-forecast -config config/application.properties

# Test with debug output
./solar-forecast -config config/application.properties -debug
```

## 4. Schedule with Cron

### Option A: Interactive Setup (Recommended)

```bash
make install
make cron-install
```

### Option B: Manual Setup

```bash
# Copy binary to standard location
sudo cp solar-forecast /usr/local/bin/

# Create log directory
mkdir -p ~/var/log

# Edit crontab
crontab -e
```

Add this line to run hourly from 6am-6pm:

```cron
0 6-18 * * * /usr/local/bin/solar-forecast -config ~/Workspace/repos/solar-forecast/config/application.properties >> ~/var/log/solar-forecast.log 2>&1
```

### Verify Setup

```bash
# List your cron jobs
crontab -l

# Check logs
tail -f ~/var/log/solar-forecast.log

# Check alert state
cat ~/.solar-forecast/alert_state.json
```

## 5. Testing the Email

Before relying on this for production use, you should test that emails work:

```bash
# Manually trigger from command line (for testing)
./solar-forecast -config config/application.properties -debug

# Check email arrives
# If it doesn't, verify:
# - Gmail app password is correct (copy exactly with spaces)
# - 2-Step Verification is enabled
# - Less secure app access is allowed
```

## Alert Behavior

- **Checks**: Every hour during `daytime_start_hour` to `daytime_end_hour`
- **Email**: Sent at most once per calendar day
- **Reset**: Daily at midnight (automatic)
- **Forecast**: Analyzes next 48 hours
- **Window**: Only checks hours between `analysis_window_start` and `analysis_window_end`

## Troubleshooting

### "No alert sent even though weather is bad"

1. Check analysis window:
   ```bash
   ./solar-forecast -debug -config config/application.properties
   ```
   Look for "Forecast analysis complete" line

2. Verify thresholds are realistic for your region
3. Check current time is within `daytime_start_hour` to `daytime_end_hour`
4. Check alert state file: `cat ~/.solar-forecast/alert_state.json`

### "Gmail authentication failed"

1. Verify app password (not regular password):
   - Go to https://myaccount.google.com/apppasswords
   - Select Mail + your device
   - Copy the 16-character password (with spaces)
   - Paste exactly into `config/application.properties`

2. Ensure 2-Step Verification is enabled

### "API timeout or connection error"

1. Verify internet connection: `ping api.open-meteo.com`
2. Check firewall isn't blocking HTTPS
3. Increase `api_timeout_seconds` if your connection is slow
4. Increase `api_retry_attempts` for unreliable networks

### "Alert state not resetting"

Delete the state file to reset:

```bash
rm ~/.solar-forecast/alert_state.json
```

It will be recreated on next run.

## Monitoring

### Watch logs in real-time

```bash
tail -f ~/var/log/solar-forecast.log
```

### Check last alert

```bash
cat ~/.solar-forecast/alert_state.json
```

### Verify cron is running

```bash
# macOS: check System Report > System Metrics > Power
# Or look at syslog:
log stream --predicate 'process == "solar-forecast"'
```

## Performance Optimization

- **API Response Time**: ~500ms-2s (no retries needed usually)
- **Total Runtime**: ~5-10 seconds
- **Memory**: ~10MB
- **CPU**: Minimal (I/O bound)

Safe to run every hour without impacting system.

## Data Privacy

- Your Gmail app password stored locally in `config/application.properties`
- Keep this file secure (consider `chmod 600 config/application.properties`)
- Alert state stored locally at `~/.solar-forecast/alert_state.json`
- All data stays on your machine (Open-Meteo API is public, no credentials sent)
- No tracking or analytics

---

**Questions?** Check the full README.md for detailed documentation.
