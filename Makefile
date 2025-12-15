.PHONY: build run clean test help install cron-install

# Default target
help:
	@echo "Solar Forecast Warning System - Make targets"
	@echo ""
	@echo "  make build           - Build the binary"
	@echo "  make run             - Run the application locally"
	@echo "  make run-debug       - Run with debug logging"
	@echo "  make test            - Run tests (if available)"
	@echo "  make clean           - Remove binary and logs"
	@echo "  make install         - Install binary to /usr/local/bin"
	@echo "  make cron-install    - Set up cron job (interactive)"
	@echo ""

# Build the binary
build:
	@echo "Building solar-forecast..."
	go build -o solar-forecast ./cmd/solar-forecast
	@echo "✓ Built successfully: solar-forecast"

# Run the application
run: build
	@echo "Running solar-forecast..."
	./solar-forecast -config config/application.properties

# Run with debug logging
run-debug: build
	@echo "Running solar-forecast with debug logging..."
	./solar-forecast -config config/application.properties -debug

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f solar-forecast
	@echo "✓ Clean complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Install to system
install: build
	@echo "Installing solar-forecast to /usr/local/bin..."
	sudo cp solar-forecast /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/solar-forecast"
	@echo ""
	@echo "Next steps:"
	@echo "1. Edit config/application.properties with your settings"
	@echo "2. Run 'make cron-install' to set up cron scheduling"

# Interactive cron setup
cron-install:
	@echo "Solar Forecast Cron Installation"
	@echo "=================================="
	@echo ""
	@echo "This will set up an hourly cron job during daytime hours."
	@echo ""
	@read -p "Enter the full path to solar-forecast binary (default: /usr/local/bin/solar-forecast): " BINARY; \
	BINARY=$${BINARY:-/usr/local/bin/solar-forecast}; \
	@read -p "Enter config file path (default: ~/Workspace/repos/solar-forecast/config/application.properties): " CONFIG; \
	CONFIG=$${CONFIG:-~/Workspace/repos/solar-forecast/config/application.properties}; \
	@read -p "Enter log file path (default: ~/var/log/solar-forecast.log): " LOG; \
	LOG=$${LOG:-~/var/log/solar-forecast.log}; \
	@echo ""; \
	@echo "Creating cron entry..."; \
	@echo ""; \
	@echo "Cron job: 0 6-18 * * * $$BINARY -config $$CONFIG >> $$LOG 2>&1"; \
	@echo ""; \
	@echo "This will run: hourly from 6am to 6pm, every day"; \
	@echo ""; \
	(crontab -l 2>/dev/null; echo "0 6-18 * * * $$BINARY -config $$CONFIG >> $$LOG 2>&1") | crontab -; \
	@echo "✓ Cron job installed successfully!"; \
	@echo ""; \
	@echo "To verify: crontab -l"; \
	@echo "To remove: crontab -e (then delete the line)"; \
	@mkdir -p ~/var/log
