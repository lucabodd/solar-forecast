package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b0d/solar-forecast/internal/domain"
)

// FileStateAdapter implements AlertStateRepository using file-based storage
type FileStateAdapter struct {
	stateFile string
	logger    domain.Logger
}

// stateData represents the persistent state structure
type stateData struct {
	LastAlertDate     string `json:"last_alert_date"` // ISO 8601 format
	AlertSent         bool   `json:"alert_sent"`
	AlertRecovered    bool   `json:"alert_recovered"`
	RecoveryEmailSent bool   `json:"recovery_email_sent"`
}

// NewFileStateAdapter creates a new file-based state adapter
func NewFileStateAdapter(stateFilePath string, logger domain.Logger) *FileStateAdapter {
	// Ensure directory exists
	dir := filepath.Dir(stateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("Failed to create state directory", "error", err.Error())
	}

	return &FileStateAdapter{
		stateFile: stateFilePath,
		logger:    logger,
	}
}

// GetLastAlertDate retrieves the last alert date from file
func (f *FileStateAdapter) GetLastAlertDate(ctx context.Context) (domain.AlertState, error) {
	state := domain.AlertState{
		AlertSent: false,
	}

	// Check if file exists
	if _, err := os.Stat(f.stateFile); os.IsNotExist(err) {
		f.logger.Debug("State file does not exist, returning empty state")
		return state, nil
	}

	// Read file
	data, err := os.ReadFile(f.stateFile)
	if err != nil {
		f.logger.Error("Failed to read state file", "error", err.Error())
		return state, fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse JSON
	var stored stateData
	if err := json.Unmarshal(data, &stored); err != nil {
		f.logger.Error("Failed to parse state file", "error", err.Error())
		return state, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Parse date with local timezone
	if stored.LastAlertDate != "" {
		lastDate, err := time.ParseInLocation("2006-01-02", stored.LastAlertDate, time.Local)
		if err != nil {
			f.logger.Error("Failed to parse last alert date", "error", err.Error(), "date", stored.LastAlertDate)
			return state, fmt.Errorf("failed to parse last alert date: %w", err)
		}
		state.LastAlertDate = lastDate
	}

	state.AlertSent = stored.AlertSent
	state.AlertRecovered = stored.AlertRecovered
	state.RecoveryEmailSent = stored.RecoveryEmailSent

	f.logger.Debug("Retrieved alert state", "last_alert_date", stored.LastAlertDate, "alert_sent", stored.AlertSent, "recovery_email_sent", stored.RecoveryEmailSent)
	return state, nil
}

// SaveAlertDate saves the current alert sent date to file
func (f *FileStateAdapter) SaveAlertDate(ctx context.Context, state domain.AlertState) error {
	data := stateData{
		LastAlertDate:     state.LastAlertDate.Format("2006-01-02"),
		AlertSent:         state.AlertSent,
		AlertRecovered:    state.AlertRecovered,
		RecoveryEmailSent: state.RecoveryEmailSent,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		f.logger.Error("Failed to marshal state data", "error", err.Error())
		return fmt.Errorf("failed to marshal state data: %w", err)
	}

	if err := os.WriteFile(f.stateFile, jsonData, 0644); err != nil {
		f.logger.Error("Failed to write state file", "error", err.Error())
		return fmt.Errorf("failed to write state file: %w", err)
	}

	f.logger.Debug("Saved alert state", "last_alert_date", data.LastAlertDate, "alert_sent", data.AlertSent)
	return nil
}

// ResetIfNewDay checks if it's a new calendar day and resets alert state if needed
func (f *FileStateAdapter) ResetIfNewDay(ctx context.Context) (bool, error) {
	state, err := f.GetLastAlertDate(ctx)
	if err != nil {
		return false, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// If last alert was on a different date, reset
	if !state.LastAlertDate.IsZero() {
		lastAlertDate := time.Date(state.LastAlertDate.Year(), state.LastAlertDate.Month(), state.LastAlertDate.Day(), 0, 0, 0, 0, state.LastAlertDate.Location())
		if lastAlertDate.Before(today) {
			f.logger.Info("New day detected, resetting alert state", "today", today.Format("2006-01-02"), "last_alert", lastAlertDate.Format("2006-01-02"))
			// Clear the alert state for new day
			resetState := domain.AlertState{
				LastAlertDate:     now,
				AlertSent:         false,
				AlertRecovered:    false,
				RecoveryEmailSent: false,
			}
			if err := f.SaveAlertDate(ctx, resetState); err != nil {
				f.logger.Error("Failed to reset alert state", "error", err.Error())
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
}

// ShouldSendAlert checks if alert should be sent based on state and time
func (f *FileStateAdapter) ShouldSendAlert(ctx context.Context) (bool, error) {
	state, err := f.GetLastAlertDate(ctx)
	if err != nil {
		f.logger.Error("Failed to get alert state", "error", err.Error())
		return false, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// If no alert sent yet, or last alert was on a different day, allow sending
	if !state.AlertSent {
		return true, nil
	}

	if state.LastAlertDate.IsZero() {
		return true, nil
	}

	lastAlertDate := time.Date(state.LastAlertDate.Year(), state.LastAlertDate.Month(), state.LastAlertDate.Day(), 0, 0, 0, 0, state.LastAlertDate.Location())
	if lastAlertDate.Before(today) {
		f.logger.Info("New day, allowing alert to be sent")
		return true, nil
	}

	f.logger.Debug("Alert already sent today, skipping")
	return false, nil
}

// MarkAlertSent marks that alert was sent today
func (f *FileStateAdapter) MarkAlertSent(ctx context.Context) error {
	// Get current state to preserve recovery fields
	state, err := f.GetLastAlertDate(ctx)
	if err != nil {
		f.logger.Error("Failed to get current state", "error", err.Error())
		// If error getting state, just save minimal state
		state = domain.AlertState{}
	}

	// Update only the alert fields, preserving recovery fields
	state.LastAlertDate = time.Now()
	state.AlertSent = true

	return f.SaveAlertDate(ctx, state)
}

// ShouldSendRecoveryEmail checks if recovery email should be sent
func (f *FileStateAdapter) ShouldSendRecoveryEmail(ctx context.Context) (bool, error) {
	state, err := f.GetLastAlertDate(ctx)
	if err != nil {
		f.logger.Error("Failed to get alert state", "error", err.Error())
		return false, err
	}

	// Only send recovery email if alert was previously sent and recovery email hasn't been sent yet
	shouldSend := state.AlertSent && !state.RecoveryEmailSent

	f.logger.Debug("Recovery email eligibility check",
		"alert_sent", state.AlertSent,
		"recovery_email_sent", state.RecoveryEmailSent,
		"should_send", shouldSend)

	if !shouldSend {
		if !state.AlertSent {
			f.logger.Debug("Recovery email not needed - no alert was triggered")
		} else if state.RecoveryEmailSent {
			f.logger.Debug("Recovery email already sent today")
		}
		return false, nil
	}

	return true, nil
}

// MarkRecoveryEmailSent marks that recovery email has been sent
func (f *FileStateAdapter) MarkRecoveryEmailSent(ctx context.Context) error {
	state, err := f.GetLastAlertDate(ctx)
	if err != nil {
		return err
	}

	state.RecoveryEmailSent = true
	return f.SaveAlertDate(ctx, state)
}
