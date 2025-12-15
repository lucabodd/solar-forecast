# Installation & Setup Guide

## ✅ Prerequisites

- Go 1.25+ (already available)
- macOS/Linux/Windows with bash
- Gmail account with app password
- Your solar system location (latitude/longitude)

## 📍 Step 1: Get Your Gmail App Password

1. Go to [https://myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords)
2. If prompted, enable 2-Step Verification first
3. Select "Mail" and your device type
4. Google will generate a 16-character password with spaces
5. **Copy this password exactly** (including spaces)
6. Keep this password safe - it goes in your config file

## 🛠️ Step 2: Configure the Application

Edit `config/application.properties`:

```bash
nano config/application.properties
```

**Required fields (must change these):**

```properties
latitude=YOUR_LAT                    # Find on Google Maps (right-click)
longitude=YOUR_LON                   # E.g., 52.52, 13.41

gmail_sender=your-email@gmail.com    # Your Gmail address
recipient_email=your-email@gmail.com # Where alerts go
gmail_app_password=xxxx xxxx xxxx xxxx  # Your 16-char password
```

**Optional (use defaults or customize):**

```properties
# Alert thresholds
cloud_cover_threshold=80              # % clouds (0-100)
ghi_threshold=200                     # W/m² solar radiation
output_percentage_threshold=30        # % of system capacity

# Analysis window (what hours to check)
analysis_window_start=10              # Start hour (10am)
analysis_window_end=16                # End hour (4pm)

# Your solar system
rated_capacity_kw=5.0                 # System size in kW
panel_efficiency=0.20                 # 20% typical
inverter_efficiency=0.97              # 97% typical
temp_coefficient=-0.4                 # -0.4% per °C

# Cron schedule
daytime_start_hour=6                  # Cron starts (6am)
daytime_end_hour=18                   # Cron stops (6pm)

# API resilience
api_retry_attempts=3
api_retry_delay_seconds=5
api_timeout_seconds=10
```

## 🧪 Step 3: Test Locally

Run the application manually to verify everything works:

```bash
# Basic test
./solar-forecast -config config/application.properties

# With debug output (shows detailed logs)
./solar-forecast -config config/application.properties -debug
```

**Expected output:**
```
[INFO] Solar Forecast Warning System started version=1.0
[INFO] Configuration loaded successfully
[INFO] Successfully fetched forecast from Open-Meteo
[INFO] Forecast analysis complete
[INFO] Alert email sent successfully
```

**Check your email inbox** - If configured correctly, you should receive a test email within a few seconds.

## ⏰ Step 4: Schedule with Cron

### Option A: Automatic Setup (Recommended)

```bash
chmod +x scripts/install-cron.sh
./scripts/install-cron.sh
```

### Option B: Manual Setup

```bash
sudo cp solar-forecast /usr/local/bin/
mkdir -p ~/var/log
crontab -e
```

Add this line:
```cron
0 6-18 * * * /usr/local/bin/solar-forecast -config ~/Workspace/repos/solar-forecast/config/application.properties >> ~/var/log/solar-forecast.log 2>&1
```

## ✅ Verification

```bash
# Check cron is installed
crontab -l

# Watch logs
tail -f ~/var/log/solar-forecast.log

# Check alert state
cat ~/.solar-forecast/alert_state.json
```

## 🔐 Security

```bash
# Protect config file
chmod 600 config/application.properties

# Don't commit secrets
# (Already in .gitignore)
```

For detailed information, see: README.md, QUICKSTART.md, PROJECT.md
