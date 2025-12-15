#!/bin/bash
# Solar Forecast Cron Installation Script

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║   Solar Forecast Warning System - Cron Installation       ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Get current directory (where script is located)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_DIR="$( dirname "$SCRIPT_DIR" )"

# Default paths
DEFAULT_BINARY="$PROJECT_DIR/solar-forecast"
DEFAULT_CONFIG="$PROJECT_DIR/config/application.properties"
DEFAULT_LOG="$HOME/var/log/solar-forecast.log"
DEFAULT_STATE="$HOME/.solar-forecast"

echo "This script will set up an hourly cron job to check solar forecasts"
echo "and send email alerts when production is forecasted to be low."
echo ""

# Check if binary exists
if [ ! -f "$DEFAULT_BINARY" ]; then
    echo -e "${RED}✗ Binary not found at $DEFAULT_BINARY${NC}"
    echo "Please build the binary first: make build"
    exit 1
fi

# Get user inputs with defaults
read -p "Binary path [${DEFAULT_BINARY}]: " BINARY
BINARY=${BINARY:-$DEFAULT_BINARY}

read -p "Config file path [${DEFAULT_CONFIG}]: " CONFIG
CONFIG=${CONFIG:-$DEFAULT_CONFIG}

read -p "Log file path [${DEFAULT_LOG}]: " LOGFILE
LOGFILE=${LOGFILE:-$DEFAULT_LOG}

read -p "State directory [${DEFAULT_STATE}]: " STATE_DIR
STATE_DIR=${STATE_DIR:-$DEFAULT_STATE}

# Validate config file exists
if [ ! -f "$CONFIG" ]; then
    echo -e "${RED}✗ Config file not found: $CONFIG${NC}"
    exit 1
fi

# Create directories if they don't exist
mkdir -p "$(dirname "$LOGFILE")"
mkdir -p "$STATE_DIR"

# Display summary
echo ""
echo -e "${GREEN}Configuration Summary${NC}"
echo "─────────────────────────────────────────────────────────"
echo "Binary:        $BINARY"
echo "Config:        $CONFIG"
echo "Log file:      $LOGFILE"
echo "State dir:     $STATE_DIR"
echo ""
echo -e "${YELLOW}Cron Schedule${NC}"
echo "─────────────────────────────────────────────────────────"
echo "Frequency:     Hourly (0 * * * *)"
echo "Hours:         6am - 6pm (0 6-18 * * *)"
echo "Frequency:     Every day"
echo ""

# Show the cron command
CRON_CMD="0 6-18 * * * $BINARY -config $CONFIG -state $STATE_DIR >> $LOGFILE 2>&1"

echo -e "${YELLOW}Cron Entry${NC}"
echo "─────────────────────────────────────────────────────────"
echo "$CRON_CMD"
echo ""

# Confirm before installing
read -p "Install this cron job? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Installation cancelled."
    exit 0
fi

# Install cron job
echo ""
echo "Installing cron job..."

# Add to crontab (append if not already present)
if crontab -l 2>/dev/null | grep -q "solar-forecast"; then
    echo -e "${YELLOW}⚠ Cron job already exists${NC}"
    echo "Skipping to avoid duplicates."
    crontab -l | grep "solar-forecast"
else
    (crontab -l 2>/dev/null || true; echo "$CRON_CMD") | crontab -
    echo -e "${GREEN}✓ Cron job installed successfully!${NC}"
fi

echo ""
echo -e "${GREEN}Installation Complete${NC}"
echo "─────────────────────────────────────────────────────────"
echo ""
echo "Next steps:"
echo "  1. Verify setup:  crontab -l"
echo "  2. Check logs:    tail -f $LOGFILE"
echo "  3. Check state:   cat $STATE_DIR/alert_state.json"
echo ""
echo "To remove the cron job:"
echo "  crontab -e  (then delete the solar-forecast line)"
echo ""
echo "For debugging, run manually:"
echo "  $BINARY -config $CONFIG -debug"
echo ""
