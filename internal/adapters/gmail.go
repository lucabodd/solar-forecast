package adapters

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/b0d/solar-forecast/internal/domain"
)

// GmailAdapter implements EmailNotifier using Gmail SMTP
type GmailAdapter struct {
	senderEmail      string
	senderPassword   string
	recipientEmail   string
	logger           domain.Logger
}

// NewGmailAdapter creates a new Gmail adapter
func NewGmailAdapter(config *domain.Config, logger domain.Logger) *GmailAdapter {
	return &GmailAdapter{
		senderEmail:      config.GmailSender,
		senderPassword:   config.GmailAppPassword,
		recipientEmail:   config.RecipientEmail,
		logger:           logger,
	}
}

// SendAlert sends an HTML-formatted alert email with graphs
func (a *GmailAdapter) SendAlert(ctx context.Context, analysis *domain.AlertAnalysis) error {
	if !analysis.CriteriaTriggered.AnyTriggered {
		a.logger.Info("No alert criteria triggered, skipping email")
		return nil
	}

	subject := "⚠️ Solar Production Low - Weather Alert"
	htmlBody := a.generateHTMLBody(analysis)

	msg := a.formatMessage(subject, htmlBody)

	auth := smtp.PlainAuth("", a.senderEmail, a.senderPassword, "smtp.gmail.com")
	err := smtp.SendMail("smtp.gmail.com:587", auth, a.senderEmail, []string{a.recipientEmail}, msg)
	if err != nil {
		a.logger.Error("Failed to send email", "error", err.Error())
		return fmt.Errorf("failed to send alert email: %w", err)
	}

	a.logger.Info("Alert email sent successfully", "recipient", a.recipientEmail)
	return nil
}

// formatMessage creates the complete MIME message
func (a *GmailAdapter) formatMessage(subject, htmlBody string) []byte {
	var buf bytes.Buffer
	buf.WriteString("From: " + a.senderEmail + "\r\n")
	buf.WriteString("To: " + a.recipientEmail + "\r\n")
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	return buf.Bytes()
}

// generateHTMLBody generates the HTML email body with line charts and information
func (a *GmailAdapter) generateHTMLBody(analysis *domain.AlertAnalysis) string {
	var html strings.Builder

	html.WriteString(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #2c3e50; background: #ecf0f1; }
        .container { max-width: 900px; margin: 0 auto; padding: 0; }
        .header { 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            color: white; 
            padding: 40px 20px; 
            text-align: center;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .header h1 { font-size: 32px; margin-bottom: 5px; font-weight: 600; }
        .header .timestamp { font-size: 14px; opacity: 0.9; }
        
        .content { background: white; padding: 30px 20px; }
        
        .alert-banner {
            background: linear-gradient(135deg, #ff6b6b 0%, #ee5a6f 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 4px 6px rgba(255, 107, 107, 0.2);
        }
        .alert-banner h2 { font-size: 20px; margin-bottom: 5px; }
        .alert-banner p { opacity: 0.95; }
        
        .metrics { 
            display: grid; 
            grid-template-columns: 1fr 1fr; 
            gap: 15px; 
            margin-bottom: 30px;
        }
        .metric { 
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            padding: 20px; 
            border-radius: 8px; 
            text-align: center;
            box-shadow: 0 2px 4px rgba(0,0,0,0.05);
        }
        .metric-label { 
            font-size: 12px; 
            color: #7f8c8d; 
            text-transform: uppercase; 
            letter-spacing: 0.5px;
            margin-bottom: 8px; 
            font-weight: 600;
        }
        .metric-value { 
            font-size: 28px; 
            font-weight: 700; 
            color: #2c3e50; 
        }
        .metric-value.large { font-size: 32px; }
        .metric.triggered { 
            background: linear-gradient(135deg, #ff9999 0%, #ff6b6b 100%);
        }
        .metric.triggered .metric-value { 
            color: #c0392b; 
        }
        .metric.triggered .metric-label {
            color: #a93226;
        }
        
        .chart-section {
            margin: 30px 0;
            background: linear-gradient(to bottom, #ffffff, #f8f9fa);
            padding: 25px;
            border-radius: 8px;
            border: 1px solid #e0e6ed;
            box-shadow: 0 2px 4px rgba(0,0,0,0.05);
        }
        .chart-title {
            font-size: 18px;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #667eea;
        }
        
        svg { width: 100%; height: auto; }
        
        .details { 
            background: linear-gradient(135deg, #e8f4f8 0%, #f5f9fb 100%);
            padding: 25px; 
            border-radius: 8px; 
            margin: 30px 0;
            border-left: 4px solid #667eea;
        }
        .details h3 { 
            margin-bottom: 15px; 
            color: #2c3e50;
            font-size: 16px;
        }
        .detail-item { 
            padding: 10px 0; 
            border-bottom: 1px solid #d5dce0;
            color: #34495e;
            font-size: 14px;
        }
        .detail-item:last-child { border-bottom: none; }
        .detail-item strong { color: #2c3e50; font-weight: 600; }
        
        .recommendation { 
            background: linear-gradient(135deg, #d4edda 0%, #c8e6c9 100%);
            border-left: 4px solid #28a745; 
            padding: 25px; 
            margin: 30px 0; 
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(40, 167, 69, 0.1);
        }
        .recommendation h3 { 
            margin-bottom: 10px; 
            color: #155724;
            font-size: 16px;
        }
        .recommendation p { 
            color: #155724;
            line-height: 1.8;
        }
        
        .footer { 
            text-align: center; 
            color: #7f8c8d; 
            font-size: 12px; 
            margin-top: 40px; 
            padding: 20px;
            border-top: 1px solid #bdc3c7;
        }
        .footer p { margin: 5px 0; }
    </style>
    <script>
        // Simple SVG line chart generation
        function generateLineChart(data, height, yMax, color) {
            const padding = 40;
            const width = 800;
            const chartWidth = width - padding * 2;
            const chartHeight = height - padding * 2;
            
            if (data.length === 0) return '';
            
            const pointSpacing = chartWidth / (data.length - 1 || 1);
            let pathData = 'M';
            
            data.forEach((value, i) => {
                const x = padding + i * pointSpacing;
                const y = height - padding - (value / yMax) * chartHeight;
                pathData += (i === 0 ? '' : ' L') + x + ' ' + y;
            });
            
            let areaData = 'M' + padding + ' ' + (height - padding);
            data.forEach((value, i) => {
                const x = padding + i * pointSpacing;
                const y = height - padding - (value / yMax) * chartHeight;
                areaData += ' L' + x + ' ' + y;
            });
            areaData += ' L' + (width - padding) + ' ' + (height - padding) + ' Z';
            
            return pathData + '|' + areaData + '|' + color;
        }
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>☀️ Solar Production Alert</h1>
            <div class="timestamp">` + time.Now().Format("Monday, January 2 • 15:04 MST") + `</div>
        </div>

        <div class="content">
            <div class="alert-banner">
                <h2>⚠️ Low Solar Production Forecasted</h2>
                <p>The next 48 hours show significant reduction in solar production due to adverse weather conditions. Please review the forecast data below.</p>
            </div>

            <div class="metrics">
`)

	// Duration-based criterion display
	if analysis.CriteriaTriggered.LowProductionDurationTriggered {
		html.WriteString(fmt.Sprintf(`
                <div class="metric triggered">
                    <div class="metric-label">⚡ Production < 2kW</div>
                    <div class="metric-value">%d HOURS</div>
                </div>
`, analysis.ConsecutiveHourCount))
	} else {
		html.WriteString(`
                <div class="metric">
                    <div class="metric-label">⚡ Production < 2kW</div>
                    <div class="metric-value">✓ OK</div>
                </div>
`)
	}

	html.WriteString(fmt.Sprintf(`
                <div class="metric">
                    <div class="metric-label">⏰ Time Window</div>
                    <div class="metric-value large">%s-%s</div>
                </div>
            </div>
`, analysis.FirstLowProductionHour.Format("15:04"), analysis.LastLowProductionHour.Format("15:04")))

	// Solar output line chart for low production hours
	if len(analysis.LowProductionHours) > 0 {
		html.WriteString(a.generateOutputLineChart(analysis.LowProductionHours))
	}

	// Detailed table
	html.WriteString(a.generateDetailedTable(analysis))

	// Recommendation
	html.WriteString(fmt.Sprintf(`
            <div class="recommendation">
                <h3>📋 Recommended Action</h3>
                <p>%s</p>
            </div>
`, analysis.RecommendedAction))

	html.WriteString(`
            <div class="footer">
                <p><strong>Solar Forecast Warning System</strong> | Hourly Monitoring</p>
                <p>Forecasts provided by Open-Meteo API | Accuracy: ±15-20%</p>
                <p>This email was sent automatically. Please adjust thresholds in application.properties if needed.</p>
            </div>
        </div>
    </div>
</body>
</html>
`)

	return html.String()
}

// generateCloudCoverLineChart generates an SVG line chart for cloud cover
func (a *GmailAdapter) generateCloudCoverLineChart(hours []domain.ForecastHour) string {
	var html strings.Builder
	
	// Sort by hour and limit to next 12 hours
	sort.Slice(hours, func(i, j int) bool {
		return hours[i].Hour.Before(hours[j].Hour)
	})

	if len(hours) == 0 {
		return ""
	}

	// Limit to 12 hours
	if len(hours) > 12 {
		hours = hours[:12]
	}

	chartWidth := 800
	chartHeight := 360
	padding := 60
	
	// Create point data
	var points []float64
	for i, hour := range hours {
		if i >= 12 {
			break
		}
		points = append(points, float64(hour.CloudCover))
	}

	maxValue := float64(100)
	minValue := float64(0)

	html.WriteString(`
            <div class="chart-section">
                <div class="chart-title">☁️ Cloud Cover Forecast (Next 12 Hours)</div>
                <svg viewBox="0 0 ` + fmt.Sprintf("%d %d", chartWidth, chartHeight) + `" xmlns="http://www.w3.org/2000/svg">
                    <defs>
                        <linearGradient id="cloudGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                            <stop offset="0%" style="stop-color:#667eea;stop-opacity:0.3" />
                            <stop offset="100%" style="stop-color:#667eea;stop-opacity:0.05" />
                        </linearGradient>
                        <style>
                            .chart-line { fill: none; stroke: #667eea; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
                            .chart-area { fill: url(#cloudGradient); }
                            .chart-grid { stroke: #e0e6ed; stroke-width: 1; }
                            .chart-label { font-size: 12px; fill: #7f8c8d; }
                            .chart-value { font-size: 11px; fill: #667eea; font-weight: bold; }
                            .chart-dot { fill: #667eea; r: 4; }
                        </style>
                    </defs>
                    
                    <!-- Grid lines -->
`)

	// Add grid lines
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight-2*padding)
		html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, y, chartWidth-padding, y))
		value := maxValue - (float64(i)/4.0)*(maxValue-minValue)
		html.WriteString(fmt.Sprintf(`                    <text class="chart-label" x="%d" y="%.0f" text-anchor="end">%.0f%%</text>
`, padding-10, y+4, value))
	}

	// Calculate path data
	pointSpacing := float64(chartWidth-2*padding) / float64(len(points)-1)
	pathData := fmt.Sprintf("M %d %d", padding, chartHeight-padding)
	var areaPath strings.Builder
	areaPath.WriteString(fmt.Sprintf("M %d %d", padding, chartHeight-padding))

	for i, point := range points {
		x := float64(padding) + float64(i)*pointSpacing
		y := float64(chartHeight-padding) - ((point-minValue)/(maxValue-minValue))*float64(chartHeight-2*padding)
		pathData += fmt.Sprintf(" L %.1f %.1f", x, y)
		areaPath.WriteString(fmt.Sprintf(" L %.1f %.1f", x, y))
	}
	
	areaPath.WriteString(fmt.Sprintf(" L %d %d Z", chartWidth-padding, chartHeight-padding))

	// Add area under curve
	html.WriteString(fmt.Sprintf(`                    <path class="chart-area" d="%s" />
`, areaPath.String()))

	// Add line
	html.WriteString(fmt.Sprintf(`                    <path class="chart-line" d="%s" />
`, pathData))

	// Add points and labels
	for i, point := range points {
		if i%4 == 0 || i == len(points)-1 { // Show every 4th point to avoid clutter
			x := float64(padding) + float64(i)*pointSpacing
			y := float64(chartHeight-padding) - ((point-minValue)/(maxValue-minValue))*float64(chartHeight-2*padding)
			html.WriteString(fmt.Sprintf(`                    <circle class="chart-dot" cx="%.1f" cy="%.1f" />
`, x, y))
			html.WriteString(fmt.Sprintf(`                    <text class="chart-value" x="%.1f" y="%.1f" text-anchor="middle">%.0f%%</text>
`, x, y-12, point))
		}
	}

	// Add x-axis labels (time) - every hour with larger font
	html.WriteString(`                    <!-- X-axis labels -->
`)
	for i := 0; i < len(points); i++ { // Show every hour
		if i < len(hours) {
			x := float64(padding) + float64(i)*pointSpacing
			hour := hours[i]
			// Format: HH:00 (e.g., 14:00)
			timeStr := hour.Hour.Format("15:00")
			html.WriteString(fmt.Sprintf(`                    <text x="%.1f" y="%.0f" text-anchor="middle" style="font-size: 11px; fill: #2c3e50; font-weight: bold;">%s</text>
`, x, float64(chartHeight-padding+30), timeStr))
		}
	}

	// Add x-axis line
	html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, chartHeight-padding, chartWidth-padding, chartHeight-padding))

	html.WriteString(`
                </svg>
            </div>
`)
	return html.String()
}

// generateGHILineChart generates an SVG line chart for solar irradiance (GHI)
func (a *GmailAdapter) generateGHILineChart(hours []domain.ForecastHour) string {
	var html strings.Builder
	
	// Sort by hour and limit to next 48
	sort.Slice(hours, func(i, j int) bool {
		return hours[i].Hour.Before(hours[j].Hour)
	})

	if len(hours) == 0 {
		return ""
	}

	// Limit to 12 hours
	if len(hours) > 12 {
		hours = hours[:12]
	}

	chartWidth := 800
	chartHeight := 360
	padding := 60
	
	// Create point data
	var points []float64
	var maxGHI float64
	for i, hour := range hours {
		if i >= 12 {
			break
		}
		points = append(points, hour.GlobalHorizontalIrradiance)
		if hour.GlobalHorizontalIrradiance > maxGHI {
			maxGHI = hour.GlobalHorizontalIrradiance
		}
	}

	if maxGHI == 0 {
		maxGHI = 1000
	}
	minValue := float64(0)

	html.WriteString(`
            <div class="chart-section">
                <div class="chart-title">☀️ Solar Irradiance - GHI (Next 12 Hours)</div>
                <svg viewBox="0 0 ` + fmt.Sprintf("%d %d", chartWidth, chartHeight) + `" xmlns="http://www.w3.org/2000/svg">
                    <defs>
                        <linearGradient id="ghiGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                            <stop offset="0%" style="stop-color:#ffc107;stop-opacity:0.3" />
                            <stop offset="100%" style="stop-color:#ffc107;stop-opacity:0.05" />
                        </linearGradient>
                        <style>
                            .ghi-line { fill: none; stroke: #ffc107; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
                            .ghi-area { fill: url(#ghiGradient); }
                            .chart-grid { stroke: #e0e6ed; stroke-width: 1; }
                            .chart-label { font-size: 12px; fill: #7f8c8d; }
                            .ghi-value { font-size: 11px; fill: #ffc107; font-weight: bold; }
                            .ghi-dot { fill: #ffc107; r: 4; }
                        </style>
                    </defs>
                    
                    <!-- Grid lines -->
`)

	// Add grid lines
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight-2*padding)
		html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, y, chartWidth-padding, y))
		value := maxGHI - (float64(i)/4.0)*(maxGHI-minValue)
		html.WriteString(fmt.Sprintf(`                    <text class="chart-label" x="%d" y="%.0f" text-anchor="end">%.0f</text>
`, padding-10, y+4, value))
	}

	// Calculate path data
	pointSpacing := float64(chartWidth-2*padding) / float64(len(points)-1)
	pathData := fmt.Sprintf("M %d %d", padding, chartHeight-padding)
	var areaPath strings.Builder
	areaPath.WriteString(fmt.Sprintf("M %d %d", padding, chartHeight-padding))

	for i, point := range points {
		x := float64(padding) + float64(i)*pointSpacing
		y := float64(chartHeight-padding) - ((point-minValue)/(maxGHI-minValue))*float64(chartHeight-2*padding)
		pathData += fmt.Sprintf(" L %.1f %.1f", x, y)
		areaPath.WriteString(fmt.Sprintf(" L %.1f %.1f", x, y))
	}
	
	areaPath.WriteString(fmt.Sprintf(" L %d %d Z", chartWidth-padding, chartHeight-padding))

	// Add area under curve
	html.WriteString(fmt.Sprintf(`                    <path class="ghi-area" d="%s" />
`, areaPath.String()))

	// Add line
	html.WriteString(fmt.Sprintf(`                    <path class="ghi-line" d="%s" />
`, pathData))

	// Add points and labels
	for i, point := range points {
		if i%4 == 0 || i == len(points)-1 {
			x := float64(padding) + float64(i)*pointSpacing
			y := float64(chartHeight-padding) - ((point-minValue)/(maxGHI-minValue))*float64(chartHeight-2*padding)
			html.WriteString(fmt.Sprintf(`                    <circle class="ghi-dot" cx="%.1f" cy="%.1f" />
`, x, y))
			html.WriteString(fmt.Sprintf(`                    <text class="ghi-value" x="%.1f" y="%.1f" text-anchor="middle">%.0f</text>
`, x, y-12, point))
		}
	}

	// Add x-axis labels (time) - every hour with larger font
	html.WriteString(`                    <!-- X-axis labels -->
`)
	for i := 0; i < len(points); i++ { // Show every hour
		if i < len(hours) {
			x := float64(padding) + float64(i)*pointSpacing
			hour := hours[i]
			// Format: HH:00 (e.g., 14:00)
			timeStr := hour.Hour.Format("15:00")
			html.WriteString(fmt.Sprintf(`                    <text x="%.1f" y="%.0f" text-anchor="middle" style="font-size: 11px; fill: #2c3e50; font-weight: bold;">%s</text>
`, x, float64(chartHeight-padding+30), timeStr))
		}
	}

	// Add x-axis line
	html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, chartHeight-padding, chartWidth-padding, chartHeight-padding))

	html.WriteString(`
                </svg>
            </div>
`)
	return html.String()
}

// generateOutputLineChart generates an SVG line chart for estimated solar output
func (a *GmailAdapter) generateOutputLineChart(production []domain.SolarProduction) string {
	var html strings.Builder
	
	// Sort by hour and limit to next 12 hours
	sort.Slice(production, func(i, j int) bool {
		return production[i].Hour.Before(production[j].Hour)
	})

	if len(production) == 0 {
		return ""
	}

	// Limit to 12 hours
	if len(production) > 12 {
		production = production[:12]
	}

	chartWidth := 800
	chartHeight := 360
	padding := 60
	
	// Create point data
	var points []float64
	for i, prod := range production {
		if i >= 12 {
			break
		}
		percent := prod.OutputPercentage
		if percent < 0 {
			percent = 0
		}
		points = append(points, percent)
	}

	maxValue := float64(100)
	minValue := float64(0)

	html.WriteString(`
            <div class="chart-section">
                <div class="chart-title">⚡ Estimated Solar Output (Next 12 Hours)</div>
                <svg viewBox="0 0 ` + fmt.Sprintf("%d %d", chartWidth, chartHeight) + `" xmlns="http://www.w3.org/2000/svg">
                    <defs>
                        <linearGradient id="outputGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                            <stop offset="0%" style="stop-color:#28a745;stop-opacity:0.3" />
                            <stop offset="100%" style="stop-color:#28a745;stop-opacity:0.05" />
                        </linearGradient>
                        <style>
                            .output-line { fill: none; stroke: #28a745; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
                            .output-area { fill: url(#outputGradient); }
                            .chart-grid { stroke: #e0e6ed; stroke-width: 1; }
                            .chart-label { font-size: 12px; fill: #7f8c8d; }
                            .output-value { font-size: 11px; fill: #28a745; font-weight: bold; }
                            .output-dot { fill: #28a745; r: 4; }
                        </style>
                    </defs>
                    
                    <!-- Grid lines -->
`)

	// Add grid lines
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight-2*padding)
		html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, y, chartWidth-padding, y))
		value := maxValue - (float64(i)/4.0)*(maxValue-minValue)
		html.WriteString(fmt.Sprintf(`                    <text class="chart-label" x="%d" y="%.0f" text-anchor="end">%.0f%%</text>
`, padding-10, y+4, value))
	}

	// Calculate path data
	pointSpacing := float64(chartWidth-2*padding) / float64(len(points)-1)
	pathData := fmt.Sprintf("M %d %d", padding, chartHeight-padding)
	var areaPath strings.Builder
	areaPath.WriteString(fmt.Sprintf("M %d %d", padding, chartHeight-padding))

	for i, point := range points {
		x := float64(padding) + float64(i)*pointSpacing
		y := float64(chartHeight-padding) - ((point-minValue)/(maxValue-minValue))*float64(chartHeight-2*padding)
		pathData += fmt.Sprintf(" L %.1f %.1f", x, y)
		areaPath.WriteString(fmt.Sprintf(" L %.1f %.1f", x, y))
	}
	
	areaPath.WriteString(fmt.Sprintf(" L %d %d Z", chartWidth-padding, chartHeight-padding))

	// Add area under curve
	html.WriteString(fmt.Sprintf(`                    <path class="output-area" d="%s" />
`, areaPath.String()))

	// Add line
	html.WriteString(fmt.Sprintf(`                    <path class="output-line" d="%s" />
`, pathData))

	// Add points and labels
	for i, point := range points {
		if i%4 == 0 || i == len(points)-1 {
			x := float64(padding) + float64(i)*pointSpacing
			y := float64(chartHeight-padding) - ((point-minValue)/(maxValue-minValue))*float64(chartHeight-2*padding)
			html.WriteString(fmt.Sprintf(`                    <circle class="output-dot" cx="%.1f" cy="%.1f" />
`, x, y))
			html.WriteString(fmt.Sprintf(`                    <text class="output-value" x="%.1f" y="%.1f" text-anchor="middle">%.1f%%</text>
`, x, y-12, point))
		}
	}

	// Add x-axis labels (time) - every hour with larger font
	html.WriteString(`                    <!-- X-axis labels -->
`)
	for i := 0; i < len(points); i++ { // Show every hour
		if i < len(production) {
			x := float64(padding) + float64(i)*pointSpacing
			prod := production[i]
			// Format: HH:00 (e.g., 14:00)
			timeStr := prod.Hour.Format("15:00")
			html.WriteString(fmt.Sprintf(`                    <text x="%.1f" y="%.0f" text-anchor="middle" style="font-size: 11px; fill: #2c3e50; font-weight: bold;">%s</text>
`, x, float64(chartHeight-padding+30), timeStr))
		}
	}

	// Add x-axis line
	html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, chartHeight-padding, chartWidth-padding, chartHeight-padding))

	html.WriteString(`
                </svg>
            </div>
`)
	return html.String()
}

// generateDetailedTable generates a table with all relevant data
func (a *GmailAdapter) generateDetailedTable(analysis *domain.AlertAnalysis) string {
	var html strings.Builder
	html.WriteString(`
        <div class="details">
            <h3>📊 Analysis Summary</h3>
            <div class="detail-item">
                <strong>Analysis Period:</strong> Next 48 hours from now
            </div>
            <div class="detail-item">
                <strong>Alert Triggered:</strong> Production below 2 kW
            </div>
            <div class="detail-item">
                <strong>Duration:</strong> %d consecutive hours
            </div>
            <div class="detail-item">
                <strong>Time Window:</strong> %s to %s
            </div>
            <div class="detail-item">
                <strong>Minimum Production:</strong> %.2f kW
            </div>
`)

	minProd := 999.0
	if len(analysis.LowProductionHours) > 0 {
		for _, p := range analysis.LowProductionHours {
			if p.EstimatedOutputKW < minProd {
				minProd = p.EstimatedOutputKW
			}
		}
	}

	html.WriteString(fmt.Sprintf(`
        </div>
`,
		analysis.ConsecutiveHourCount,
		analysis.FirstLowProductionHour.Format("15:04"),
		analysis.LastLowProductionHour.Format("15:04"),
		minProd))

	return html.String()
}

// SendRecoveryEmail sends an email indicating conditions have improved and alert is cleared
func (a *GmailAdapter) SendRecoveryEmail(ctx context.Context) error {
	subject := "✅ Solar Production Alert Cleared - Conditions Recovered"
	htmlBody := a.generateRecoveryHTMLBody()

	msg := a.formatMessage(subject, htmlBody)

	auth := smtp.PlainAuth("", a.senderEmail, a.senderPassword, "smtp.gmail.com")
	err := smtp.SendMail("smtp.gmail.com:587", auth, a.senderEmail, []string{a.recipientEmail}, msg)
	if err != nil {
		a.logger.Error("Failed to send recovery email", "error", err.Error())
		return fmt.Errorf("failed to send recovery email: %w", err)
	}

	a.logger.Info("Recovery email sent successfully", "recipient", a.recipientEmail)
	return nil
}

// generateRecoveryHTMLBody generates the HTML for recovery email
func (a *GmailAdapter) generateRecoveryHTMLBody() string {
	var html strings.Builder

	html.WriteString(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #2c3e50; background: #ecf0f1; }
        .container { max-width: 900px; margin: 0 auto; padding: 0; }
        .header { 
            background: linear-gradient(135deg, #28a745 0%, #20c997 100%); 
            color: white; 
            padding: 40px 20px; 
            text-align: center;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .header h1 { font-size: 32px; margin-bottom: 5px; font-weight: 600; }
        .header .timestamp { font-size: 14px; opacity: 0.9; }
        
        .content { background: white; padding: 30px 20px; }
        
        .recovery-banner {
            background: linear-gradient(135deg, #28a745 0%, #20c997 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 4px 6px rgba(40, 167, 69, 0.2);
        }
        .recovery-banner h2 { font-size: 20px; margin-bottom: 5px; }
        .recovery-banner p { opacity: 0.95; }
        
        .status-card {
            background: linear-gradient(135deg, #d4edda 0%, #c8e6c9 100%);
            border-left: 4px solid #28a745;
            padding: 25px;
            margin: 20px 0;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(40, 167, 69, 0.1);
        }
        .status-card h3 {
            color: #155724;
            margin-bottom: 10px;
            font-size: 18px;
        }
        .status-card p {
            color: #155724;
            line-height: 1.8;
        }
        
        .detail-item {
            padding: 12px 0;
            border-bottom: 1px solid #d5dce0;
            color: #34495e;
            font-size: 14px;
        }
        .detail-item:last-child { border-bottom: none; }
        .detail-item strong { color: #2c3e50; font-weight: 600; }
        
        .footer { 
            text-align: center; 
            color: #7f8c8d; 
            font-size: 12px; 
            margin-top: 40px; 
            padding: 20px;
            border-top: 1px solid #bdc3c7;
        }
        .footer p { margin: 5px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>☀️ Solar Production Alert Cleared</h1>
            <div class="timestamp">` + time.Now().Format("Monday, January 2 • 15:04 MST") + `</div>
        </div>

        <div class="content">
            <div class="recovery-banner">
                <h2>✅ Conditions Have Improved</h2>
                <p>Solar production conditions have returned to normal and the alert has been cleared.</p>
            </div>

            <div class="status-card">
                <h3>Status Update</h3>
                <div class="detail-item">
                    <strong>Alert Status:</strong> CLEARED ✓
                </div>
                <div class="detail-item">
                    <strong>Recovery Time:</strong> ` + time.Now().Format("15:04 MST") + `
                </div>
                <div class="detail-item">
                    <strong>Conditions:</strong> Solar irradiance, cloud cover, and production levels are now within normal parameters
                </div>
                <div class="detail-item">
                    <strong>System Status:</strong> Ready for next alert cycle
                </div>
            </div>

            <div class="status-card">
                <h3>What This Means</h3>
                <p>
                    The adverse weather conditions that triggered the alert have passed. Your solar production is 
                    expected to operate normally. The system is now armed and ready to send alerts if adverse 
                    conditions are forecasted again in the future.
                </p>
            </div>

            <div class="footer">
                <p>This is an automated notification from your Solar Production Monitoring System</p>
                <p>Generated at ` + time.Now().Format("2006-01-02 15:04:05 MST") + `</p>
            </div>
        </div>
    </div>
</body>
</html>
`)

	return html.String()
}
