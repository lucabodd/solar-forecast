# Refactor Plan: Duration-Based Production Alert System

## Current Metrics Approach
The system currently triggers alerts based on **percentage-based thresholds** evaluated across analysis window:
- Cloud cover ≥ 80% (majority of hours)
- GHI ≤ 200 W/m² (majority of hours)
- Output ≤ 30% of capacity (majority of hours)
- Uses "majority logic" (> 50% of window hours must trigger)

## New Metrics Approach
Switch to **duration-based continuous monitoring**:
- Alert when estimated production **< 2 kW for 6+ consecutive hours** during daytime
- Changes from "snapshot majority" to "sustained duration" metric
- Better for detecting sustained weather events, not just bad hour snapshots

---

## Refactor Scope

### 1. Configuration Changes (`config/application.properties`)

**Remove:**
```properties
# Old percentage-based thresholds
cloud_cover_threshold=80
ghi_threshold=200
output_percentage_threshold=30
```

**Add:**
```properties
# New duration-based thresholds
production_alert_threshold_kw=2.0          # Alert if production drops below 2 kW
duration_threshold_hours=6                 # Alert if below threshold for 6+ consecutive hours
```

**Keep:** `analysis_window_start` and `analysis_window_end` for daytime window

---

### 2. Domain Model Changes (`internal/domain/models.go`)

**Update `Config` struct:**
```go
// OLD
CloudCoverThreshold      int
GHIThreshold             float64
OutputPercentageThreshold float64

// NEW
ProductionAlertThresholdKW float64 // Alert if production < this
DurationThresholdHours     int     // Alert if threshold exceeded for this many consecutive hours
```

**Update `AlertCriteria` struct:**
```go
// OLD
CloudCoverTriggered    bool
GHITriggered          bool
OutputPercentageTriggered bool
AnyTriggered          bool

// NEW
LowProductionDurationTriggered bool
AnyTriggered                   bool
```

**Update `AlertAnalysis` struct:**
```go
// Add new fields for duration-based analysis
LowProductionHours      []SolarProduction  // Hours with production < threshold
ConsecutiveHourCount    int                // How many consecutive hours triggered
FirstLowProductionHour  time.Time          // Start of low production period
LastLowProductionHour   time.Time          // End of low production period
```

---

### 3. Service Logic Changes (`internal/domain/service.go`)

#### Refactor `analyzeForecast()` method:
1. **Remove** separate criterion evaluation calls:
   - `evaluateCloudCoverCriterion()`
   - `evaluateGHICriterion()`
   - `evaluateOutputCriterion()`

2. **Add** new method: `evaluateLowProductionDuration()`
   - Loop through forecast hours in analysis window
   - Calculate production for each hour
   - Find consecutive sequences where production < threshold
   - If any sequence ≥ duration threshold → trigger alert
   - Track the longest sequence (or first one to exceed threshold)

#### Algorithm Detail:
```
For each hour in forecast window:
  1. Calculate estimated production in kW
  2. If production >= threshold:
     - Reset consecutive count to 0
  3. If production < threshold:
     - Increment consecutive count
     - Add to current sequence
  4. If consecutive count >= 6:
     - Set CriteriaTriggered.LowProductionDurationTriggered = true
     - Store sequence details (start hour, end hour, count)
     - Break (alert triggered)
```

#### Simplify `analyzeForecast()`:
- Remove "majority percentage" calculation (no longer needed)
- Remove multiple criterion evaluation
- Replace with single duration-based check
- Update recommendation text for duration context

---

### 4. Adapter Updates (Minor)

**`internal/adapters/filestate.go`:**
- No changes needed (state persistence unchanged)

**`internal/adapters/gmail.go`:**
- Update email template to show:
  - Specific start time of low production period
  - Duration count (e.g., "6+ hours")
  - Exact hours affected (e.g., "10:00-16:00")
  - Still include charts but with focus on production < 2kW

---

## Implementation Steps

1. **Update configuration file** - Add new properties, remove old ones
2. **Update domain models** - Config struct, AlertCriteria, AlertAnalysis
3. **Refactor service logic** - Remove three criterion methods, add duration method
4. **Update email adapter** - Modify email templates for new metric
5. **Update state adapter** - No changes (reuse existing persistence)
6. **Test** - Verify with sample forecast data where production dips below 2kW for 6+ hours
7. **Update documentation** - README, application properties comments

---

## Benefits of New Approach

✅ **More practical** - Detects sustained production loss (real weather events)
✅ **Clearer threshold** - Single metric: "< 2kW for 6+ hours"
✅ **Better for scheduling** - Shows exact time window of low production
✅ **Reduced false positives** - One bad hour doesn't trigger if not sustained
✅ **Simpler logic** - Single criterion instead of 3 separate ones
✅ **Better actionable** - User knows exactly when to expect low output

---

## Example Scenario

**Current approach:** Alert if 50%+ of 10:00-16:00 hours have GHI ≤ 200 W/m²
- Could trigger with scattered clouds throughout day

**New approach:** Alert if production drops below 2kW for 6+ consecutive hours
- Only triggers for sustained weather event (e.g., overcast system passing through)
- User sees "10:00-16:00: production will be < 2kW" - actionable time window

---

## Files to Modify

1. `config/application.properties` - Configuration
2. `internal/domain/models.go` - Data structures
3. `internal/domain/service.go` - Business logic (major refactor)
4. `internal/adapters/gmail.go` - Email templates (minor updates)
5. Optional: README.md - Update feature description

**No changes needed:**
- `cmd/solar-forecast/main.go` - Entry point unchanged
- `internal/adapters/openmeteo.go` - API unchanged
- `internal/adapters/filestate.go` - Persistence unchanged
- `internal/adapters/logger.go` - Logging unchanged
- `internal/config/loader.go` - Loader can remain generic
