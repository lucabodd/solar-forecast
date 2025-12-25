package adapters

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"time"

	"github.com/b0d/solar-forecast/internal/domain"
	"github.com/fogleman/gg"
)

// PushoverAdapter implements PushNotifier using Pushover API
type PushoverAdapter struct {
	userKey  string
	apiToken string
	logger   domain.Logger
}

// NewPushoverAdapter creates a new Pushover adapter
func NewPushoverAdapter(config *domain.Config, logger domain.Logger) *PushoverAdapter {
	return &PushoverAdapter{
		userKey:  config.PushoverUserKey,
		apiToken: config.PushoverAPIToken,
		logger:   logger,
	}
}

// GenerateChartImage creates a PNG image of the production and cloud coverage chart
func (p *PushoverAdapter) GenerateChartImage(production []domain.SolarProduction) ([]byte, error) {
	// Sort and filter to next 12 hours from now
	sort.Slice(production, func(i, j int) bool {
		return production[i].Hour.Before(production[j].Hour)
	})
	production = filterFromNow(production, 12)

	// Debug: log time range
	if len(production) > 0 {
		p.logger.Debug("Pushover chart time range",
			"now", time.Now().Format("2006-01-02 15:04:05"),
			"first_hour", production[0].Hour.Format("2006-01-02 15:04:05"),
			"last_hour", production[len(production)-1].Hour.Format("2006-01-02 15:04:05"),
			"total_hours", len(production))
	}

	if len(production) == 0 {
		return nil, fmt.Errorf("no production data available")
	}

	// Image dimensions
	width := 800
	height := 400
	padding := 60
	rightPadding := 80

	dc := gg.NewContext(width+rightPadding, height)

	// Background
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.Clear()

	// Calculate data ranges
	var maxProduction float64
	for _, prod := range production {
		if prod.EstimatedOutputKW > maxProduction {
			maxProduction = prod.EstimatedOutputKW
		}
	}
	maxProduction = float64(int(maxProduction) + 1)
	if maxProduction < 2 {
		maxProduction = 2
	}

	chartWidth := width - padding
	chartHeight := height - 2*padding
	maxCloud := 100.0

	// Draw grid and axes
	dc.SetColor(color.RGBA{224, 230, 237, 255})
	dc.SetLineWidth(1)
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight)
		dc.DrawLine(float64(padding), y, float64(chartWidth), y)
		dc.Stroke()
	}

	// Y-axis labels (production)
	dc.SetColor(color.RGBA{127, 140, 141, 255})
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight)
		value := maxProduction - (float64(i)/4.0)*maxProduction
		dc.DrawStringAnchored(fmt.Sprintf("%.1f kW", value), float64(padding-10), y, 1, 0.5)
	}

	// Y-axis labels (cloud coverage - right side)
	dc.SetColor(color.RGBA{52, 152, 219, 255})
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight)
		value := maxCloud - (float64(i)/4.0)*maxCloud
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", value), float64(chartWidth+10), y, 0, 0.5)
	}

	// Calculate point spacing
	pointSpacing := float64(chartWidth-padding) / float64(len(production)-1)

	// Draw production line (orange)
	dc.SetColor(color.RGBA{247, 147, 30, 255})
	dc.SetLineWidth(4)
	for i, prod := range production {
		x := float64(padding) + float64(i)*pointSpacing
		kw := prod.EstimatedOutputKW
		if kw < 0 {
			kw = 0
		}
		y := float64(padding+chartHeight) - (kw/maxProduction)*float64(chartHeight)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	dc.Stroke()

	// Draw cloud coverage line (blue, dashed)
	dc.SetColor(color.RGBA{52, 152, 219, 180})
	dc.SetLineWidth(3)
	dc.SetDash(5, 5)
	for i, prod := range production {
		x := float64(padding) + float64(i)*pointSpacing
		cloud := float64(prod.CloudCover)
		y := float64(padding+chartHeight) - (cloud/maxCloud)*float64(chartHeight)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	dc.Stroke()
	dc.SetDash() // Reset dash

	// Add data point labels for production (every point)
	dc.SetColor(color.RGBA{247, 147, 30, 255})
	for i, prod := range production {
		kw := prod.EstimatedOutputKW
		if kw < 0 {
			kw = 0
		}
		x := float64(padding) + float64(i)*pointSpacing
		y := float64(padding+chartHeight) - (kw/maxProduction)*float64(chartHeight)

		// Draw dot
		dc.DrawCircle(x, y, 4)
		dc.Fill()

		// Draw value label above the line
		dc.DrawStringAnchored(fmt.Sprintf("%.1f", kw), x, y-10, 0.5, 1)
	}

	// Add data point labels for cloud coverage (every point)
	dc.SetColor(color.RGBA{52, 152, 219, 255})
	for i, prod := range production {
		cloud := float64(prod.CloudCover)
		x := float64(padding) + float64(i)*pointSpacing
		y := float64(padding+chartHeight) - (cloud/maxCloud)*float64(chartHeight)

		// Draw dot
		dc.DrawCircle(x, y, 3)
		dc.Fill()

		// Draw value label below the line
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", cloud), x, y+15, 0.5, 0)
	}

	// X-axis labels (time) - show every hour
	dc.SetColor(color.RGBA{44, 62, 80, 255})
	for i, prod := range production {
		x := float64(padding) + float64(i)*pointSpacing
		timeStr := prod.Hour.Format("15:04")
		dc.DrawStringAnchored(timeStr, x, float64(padding+chartHeight+35), 0.5, 0)
	}

	// Title
	dc.SetColor(color.RGBA{44, 62, 80, 255})
	dc.DrawStringAnchored("Solar Production & Cloud Coverage (Next 12h)", float64(width/2), 25, 0.5, 0.5)

	// Legend
	dc.SetColor(color.RGBA{247, 147, 30, 255})
	dc.DrawStringAnchored("● Production (kW)", float64(padding+80), float64(padding-15), 0.5, 0.5)
	dc.SetColor(color.RGBA{52, 152, 219, 255})
	dc.DrawStringAnchored("● Cloud Coverage (%)", float64(padding+280), float64(padding-15), 0.5, 0.5)

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	return buf.Bytes(), nil
}

// SendNotification sends a push notification with chart image via Pushover API
func (p *PushoverAdapter) SendNotification(ctx context.Context, title, message string, imageData []byte) error {
	// Check if Pushover is configured
	if p.userKey == "" || p.apiToken == "" ||
		p.userKey == "YOUR_PUSHOVER_USER_KEY" || p.apiToken == "YOUR_PUSHOVER_API_TOKEN" {
		p.logger.Debug("Pushover not configured, skipping notification")
		return nil // Not an error, just not configured
	}

	// Create multipart form body
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add text fields
	writer.WriteField("token", p.apiToken)
	writer.WriteField("user", p.userKey)
	writer.WriteField("title", title)
	writer.WriteField("message", message)
	writer.WriteField("priority", "1")
	writer.WriteField("sound", "solar")

	// Add image attachment if provided
	if len(imageData) > 0 {
		part, err := writer.CreateFormFile("attachment", "chart.png")
		if err != nil {
			p.logger.Warn("Failed to create form file for image", "error", err.Error())
		} else {
			if _, err := part.Write(imageData); err != nil {
				p.logger.Warn("Failed to write image data", "error", err.Error())
			}
		}
	}

	writer.Close()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.pushover.net/1/messages.json", &body)
	if err != nil {
		p.logger.Error("Failed to create Pushover request", "error", err.Error())
		return fmt.Errorf("failed to create pushover request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request with timeout
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		p.logger.Error("Failed to send Pushover notification", "error", err.Error())
		return fmt.Errorf("failed to send pushover notification: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.Warn("Failed to read Pushover response", "error", err.Error())
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		p.logger.Error("Pushover API returned error",
			"status", resp.StatusCode,
			"response", string(respBody))
		return fmt.Errorf("pushover API error: %s (status %d)", string(respBody), resp.StatusCode)
	}

	// Log response for debugging
	p.logger.Debug("Pushover API response", "status", resp.StatusCode, "body", string(respBody))

	p.logger.Info("Push notification sent successfully",
		"title", title,
		"has_image", len(imageData) > 0,
		"image_size_kb", len(imageData)/1024)
	return nil
}
