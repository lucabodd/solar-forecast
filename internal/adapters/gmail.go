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
	alertThresholdKW float64 // Store threshold for color coding in emails
}

// NewGmailAdapter creates a new Gmail adapter
func NewGmailAdapter(config *domain.Config, logger domain.Logger) *GmailAdapter {
	return &GmailAdapter{
		senderEmail:      config.GmailSender,
		senderPassword:   config.GmailAppPassword,
		recipientEmail:   config.RecipientEmail,
		logger:           logger,
		alertThresholdKW: config.ProductionAlertThresholdKW,
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
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; color: #2c3e50; background: #ecf0f1; }
        .container { max-width: 900px; margin: 0 auto; padding: 0; }
        .header {
            background: linear-gradient(135deg, #FF6B35 0%, #F7931E 50%, #FFD60A 100%);
            color: white;
            padding: 50px 20px;
            text-align: center;
            box-shadow: 0 8px 16px rgba(255, 107, 53, 0.3);
            position: relative;
            overflow: hidden;
        }
        .header::before {
            content: '';
            position: absolute;
            top: -50%;
            left: -50%;
            width: 200%;
            height: 200%;
            background: radial-gradient(circle, rgba(255,255,255,0.1) 0%, transparent 70%);
            animation: rotate 20s linear infinite;
        }
        @keyframes rotate {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
        }
        .header h1 {
            font-size: 36px;
            margin-bottom: 8px;
            font-weight: 700;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
            position: relative;
            z-index: 1;
        }
        .header .timestamp {
            font-size: 15px;
            opacity: 0.95;
            font-weight: 500;
            position: relative;
            z-index: 1;
        }
        
        .content { background: white; padding: 30px 20px; }
        
        .alert-banner {
            background: linear-gradient(135deg, #FF6B6B 0%, #EE5A6F 50%, #E63946 100%);
            color: white;
            padding: 30px;
            border-radius: 12px;
            margin-bottom: 30px;
            box-shadow: 0 8px 16px rgba(255, 107, 107, 0.3);
            border-left: 5px solid #C0392B;
        }
        .alert-banner h2 {
            font-size: 24px;
            margin-bottom: 10px;
            font-weight: 700;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .alert-banner p {
            opacity: 0.95;
            font-size: 16px;
            line-height: 1.6;
        }
        
        .metrics {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .metric {
            background: linear-gradient(135deg, #FFFFFF 0%, #F8F9FA 100%);
            padding: 25px;
            border-radius: 12px;
            text-align: center;
            box-shadow: 0 4px 8px rgba(0,0,0,0.08);
            border: 1px solid #E0E6ED;
            transition: transform 0.2s ease;
        }
        .metric:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 12px rgba(0,0,0,0.12);
        }
        .metric-icon {
            font-size: 32px;
            margin-bottom: 10px;
            display: block;
        }
        .metric-label {
            font-size: 13px;
            color: #7F8C8D;
            text-transform: uppercase;
            letter-spacing: 0.8px;
            margin-bottom: 12px;
            font-weight: 700;
        }
        .metric-value {
            font-size: 32px;
            font-weight: 800;
            color: #2C3E50;
            line-height: 1.2;
        }
        .metric-value.large { font-size: 36px; }
        .metric.triggered {
            background: linear-gradient(135deg, #FFE5E5 0%, #FFD0D0 100%);
            border: 2px solid #FF6B6B;
        }
        .metric.triggered .metric-value {
            color: #C0392B;
        }
        .metric.triggered .metric-label {
            color: #A93226;
        }
        
        .chart-section {
            margin: 30px 0;
            background: linear-gradient(to bottom, #FFFFFF, #F8F9FA);
            padding: 30px;
            border-radius: 12px;
            border: 2px solid #E0E6ED;
            box-shadow: 0 4px 12px rgba(0,0,0,0.08);
        }
        .chart-title {
            font-size: 20px;
            font-weight: 700;
            color: #2C3E50;
            margin-bottom: 25px;
            padding-bottom: 15px;
            border-bottom: 3px solid transparent;
            background: linear-gradient(white, white) padding-box,
                        linear-gradient(135deg, #FF6B35, #FFD60A) border-box;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .chart-legend {
            display: flex;
            justify-content: center;
            gap: 20px;
            margin-top: 15px;
            font-size: 12px;
        }
        .legend-item {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .legend-color {
            width: 20px;
            height: 4px;
            border-radius: 2px;
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
            color: #7F8C8D;
            font-size: 13px;
            margin-top: 50px;
            padding: 30px 20px;
            border-top: 3px solid transparent;
            background: linear-gradient(white, white) padding-box,
                        linear-gradient(135deg, #FF6B35, #FFD60A) border-box;
        }
        .footer p {
            margin: 8px 0;
            line-height: 1.8;
        }
        .footer strong {
            color: #2C3E50;
            font-weight: 700;
        }
        .footer a {
            color: #FF6B35;
            text-decoration: none;
            font-weight: 600;
        }
        .footer a:hover {
            text-decoration: underline;
        }

        /* Mobile responsive styles */
        @media only screen and (max-width: 600px) {
            .header { padding: 30px 15px !important; }
            .header h1 { font-size: 24px !important; }
            .header .timestamp { font-size: 13px !important; }
            .content { padding: 20px 15px !important; }
            .alert-banner { padding: 20px !important; }
            .alert-banner h2 { font-size: 20px !important; }
            .alert-banner p { font-size: 14px !important; }
            .metrics {
                grid-template-columns: 1fr !important;
                gap: 15px !important;
            }
            .metric { padding: 20px !important; }
            .metric-value { font-size: 28px !important; }
            .metric-value.large { font-size: 32px !important; }
            .chart-section { padding: 20px 15px !important; }
            .chart-title { font-size: 16px !important; }
            table { font-size: 12px !important; display: block; overflow-x: auto; }
            table th, table td { padding: 8px !important; }
            .details, .recommendation { padding: 20px !important; }
            .footer { padding: 20px 15px !important; }
        }

        @media only screen and (max-width: 400px) {
            .header h1 { font-size: 20px !important; }
            .metric-value { font-size: 24px !important; }
            .metric-value.large { font-size: 28px !important; }
        }
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
                <p>Production forecasted below ` + fmt.Sprintf("%.1f kW for %d consecutive daylight hours starting %s",
					a.alertThresholdKW,
					analysis.ConsecutiveHourCount,
					analysis.FirstLowProductionHour.Format("Mon Jan 2, 15:04")) + `. Please review the forecast data below.</p>
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

	// Recovery forecast metric
	if analysis.HasRecovery {
		// Calculate days until recovery
		now := time.Now()
		daysUntilRecovery := int(analysis.RecoveryHour.Sub(now).Hours() / 24)

		var recoveryText string
		if daysUntilRecovery == 0 {
			recoveryText = analysis.RecoveryHour.Format("Today 15:04")
		} else if daysUntilRecovery == 1 {
			recoveryText = analysis.RecoveryHour.Format("Tomorrow 15:04")
		} else {
			recoveryText = analysis.RecoveryHour.Format("Mon Jan 2, 15:04")
		}

		html.WriteString(fmt.Sprintf(`
                <div class="metric">
                    <div class="metric-label">🌤️ Expected Recovery</div>
                    <div class="metric-value large">%s</div>
                </div>
            </div>
`, recoveryText))
	} else {
		html.WriteString(`
                <div class="metric">
                    <div class="metric-label">🌤️ Expected Recovery</div>
                    <div class="metric-value">No recovery<br/>in 7 days</div>
                </div>
            </div>
`)
	}

	// Solar production & cloud coverage chart - showing next 12 hours from now
	if len(analysis.AllProductionHours) > 0 {
		html.WriteString(a.generateOutputLineChart(analysis.AllProductionHours))
	}

	// Hourly weather conditions table removed as requested

	// Recovery forecast section
	html.WriteString(a.generateRecoverySection(analysis))

	html.WriteString(`
            <div class="footer">
                <p><strong>⚡ Solar Forecast Warning System</strong></p>
                <p>Automated solar production monitoring • Real-time weather analysis</p>
                <p>Forecasts provided by <a href="https://open-meteo.com" style="color: #FF6B35; text-decoration: none;">Open-Meteo API</a> • Accuracy: ±15-20%</p>
                <p style="margin-top: 15px; padding-top: 15px; border-top: 1px solid #E0E6ED;">
                    Generated at ` + time.Now().Format("15:04 MST") + ` • This email was sent automatically
                </p>
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
		html.WriteString(fmt.Sprintf(`                    <text class="chart-label" x="%d" y="%.0f" text-anchor="end">%.1f kWh</text>
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

// filterFromNow returns up to 'count' production hours starting from current time
func filterFromNow(hours []domain.SolarProduction, count int) []domain.SolarProduction {
	now := time.Now()
	currentHourStart := now.Truncate(time.Hour) // e.g., 14:23:45 → 14:00:00

	var filtered []domain.SolarProduction
	for _, h := range hours {
		hourStart := h.Hour.Truncate(time.Hour)
		// Include current hour and all future hours
		if !hourStart.Before(currentHourStart) { // >= comparison
			filtered = append(filtered, h)
			if len(filtered) >= count {
				break
			}
		}
	}
	return filtered
}

// calculateSmartSpacing calculates non-uniform X positions that compress nighttime hours
func calculateSmartSpacing(production []domain.SolarProduction, totalWidth float64) []float64 {
	const daylightGHIThreshold = 50.0
	const nightCompressionFactor = 0.2 // Night hours take 20% of day hour spacing

	// Calculate total "weighted" hours
	var totalWeightedHours float64
	for _, prod := range production {
		if prod.GHI >= daylightGHIThreshold {
			totalWeightedHours += 1.0 // Full weight for daylight
		} else {
			totalWeightedHours += nightCompressionFactor // Compressed weight for night
		}
	}

	// Calculate base spacing
	baseSpacing := totalWidth / totalWeightedHours

	// Calculate cumulative X positions
	xPositions := make([]float64, len(production))
	var cumulativeX float64
	for i, prod := range production {
		xPositions[i] = cumulativeX
		if prod.GHI >= daylightGHIThreshold {
			cumulativeX += baseSpacing
		} else {
			cumulativeX += baseSpacing * nightCompressionFactor
		}
	}

	return xPositions
}

// generateOutputLineChart generates a dual-axis SVG chart with production (kW) and cloud coverage (%)
func (a *GmailAdapter) generateOutputLineChart(production []domain.SolarProduction) string {
	var html strings.Builder

	// Sort by hour
	sort.Slice(production, func(i, j int) bool {
		return production[i].Hour.Before(production[j].Hour)
	})

	// Filter to next 48 hours from current time
	production = filterFromNow(production, 48)

	// Debug: log time range
	if a.logger != nil && len(production) > 0 {
		a.logger.Debug("Email chart time range",
			"now", time.Now().Format("2006-01-02 15:04:05"),
			"first_hour", production[0].Hour.Format("2006-01-02 15:04:05"),
			"last_hour", production[len(production)-1].Hour.Format("2006-01-02 15:04:05"),
			"total_hours", len(production))
	}

	if len(production) == 0 {
		return ""
	}

	chartWidth := 800
	chartHeight := 360
	padding := 60
	rightPadding := 80 // Extra padding for right Y-axis

	// Create production data (hourly kW values) - up to 48 hours
	var productionPoints []float64
	var maxProduction float64
	var cloudPoints []float64

	for _, prod := range production {
		kw := prod.EstimatedOutputKW
		if kw < 0 {
			kw = 0
		}
		productionPoints = append(productionPoints, kw)
		if kw > maxProduction {
			maxProduction = kw
		}
		cloudPoints = append(cloudPoints, float64(prod.CloudCover))
	}

	// Round up maxProduction to nearest 1 kW for cleaner axis
	maxProduction = float64(int(maxProduction)+1)
	if maxProduction < 2 {
		maxProduction = 2 // Minimum scale of 2 kW
	}
	minProduction := float64(0)

	// Cloud coverage always 0-100%
	maxCloud := 100.0
	minCloud := 0.0

	html.WriteString(`
            <div class="chart-section">
                <div class="chart-title">⚡ Solar Production & Cloud Coverage Forecast (Next 48 Hours)</div>
                <svg viewBox="0 0 ` + fmt.Sprintf("%d %d", chartWidth+rightPadding, chartHeight) + `" xmlns="http://www.w3.org/2000/svg">
                    <defs>
                        <linearGradient id="outputGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                            <stop offset="0%" style="stop-color:#FFD60A;stop-opacity:0.4" />
                            <stop offset="50%" style="stop-color:#F7931E;stop-opacity:0.2" />
                            <stop offset="100%" style="stop-color:#FF6B35;stop-opacity:0.05" />
                        </linearGradient>
                        <linearGradient id="lineGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                            <stop offset="0%" style="stop-color:#FF6B35" />
                            <stop offset="50%" style="stop-color:#F7931E" />
                            <stop offset="100%" style="stop-color:#FFD60A" />
                        </linearGradient>
                        <filter id="shadow">
                            <feDropShadow dx="0" dy="2" stdDeviation="3" flood-opacity="0.3"/>
                        </filter>
                        <style>
                            .output-line { fill: none; stroke: url(#lineGradient); stroke-width: 4; stroke-linecap: round; stroke-linejoin: round; filter: url(#shadow); }
                            .cloud-line { fill: none; stroke: #3498db; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; opacity: 0.7; stroke-dasharray: 5,5; }
                            .output-area { fill: url(#outputGradient); }
                            .chart-grid { stroke: #e0e6ed; stroke-width: 1; }
                            .chart-label { font-size: 12px; fill: #7f8c8d; }
                            .chart-label-right { font-size: 12px; fill: #3498db; }
                            .output-value { font-size: 11px; fill: #FF6B35; font-weight: bold; }
                            .cloud-value { font-size: 10px; fill: #3498db; font-weight: bold; }
                            .output-dot { fill: #FF6B35; r: 4; }
                            .cloud-dot { fill: #3498db; r: 3; }
                            .legend { font-size: 13px; font-weight: bold; }
                        </style>
                    </defs>

                    <!-- Legend -->
                    <text class="legend" x="` + fmt.Sprintf("%d", padding) + `" y="25" fill="#FF6B35">● Production (kW)</text>
                    <text class="legend" x="` + fmt.Sprintf("%d", padding+180) + `" y="25" fill="#3498db">● Cloud Coverage (%)</text>

                    <!-- Grid lines -->
`)

	// Add grid lines
	for i := 0; i <= 4; i++ {
		y := float64(padding) + (float64(i) / 4.0) * float64(chartHeight-2*padding)
		html.WriteString(fmt.Sprintf(`                    <line class="chart-grid" x1="%d" y1="%.0f" x2="%d" y2="%.0f" />
`, padding, y, chartWidth-padding, y))

		// Left Y-axis labels (Production kW)
		valueLeft := maxProduction - (float64(i)/4.0)*(maxProduction-minProduction)
		html.WriteString(fmt.Sprintf(`                    <text class="chart-label" x="%d" y="%.0f" text-anchor="end">%.1f kW</text>
`, padding-10, y+4, valueLeft))

		// Right Y-axis labels (Cloud %)
		valueRight := maxCloud - (float64(i)/4.0)*(maxCloud-minCloud)
		html.WriteString(fmt.Sprintf(`                    <text class="chart-label-right" x="%d" y="%.0f" text-anchor="start">%.0f%%</text>
`, chartWidth-padding+10, y+4, valueRight))
	}

	// Calculate smart point spacing (compress nighttime hours)
	totalChartWidth := float64(chartWidth - 2*padding)
	xPositions := calculateSmartSpacing(production, totalChartWidth)

	// Build production path
	productionPath := fmt.Sprintf("M %d %d", padding, chartHeight-padding)
	var areaPath strings.Builder
	areaPath.WriteString(fmt.Sprintf("M %d %d", padding, chartHeight-padding))

	for i, point := range productionPoints {
		x := float64(padding) + xPositions[i]
		y := float64(chartHeight-padding) - ((point-minProduction)/(maxProduction-minProduction))*float64(chartHeight-2*padding)
		productionPath += fmt.Sprintf(" L %.1f %.1f", x, y)
		areaPath.WriteString(fmt.Sprintf(" L %.1f %.1f", x, y))
	}

	areaPath.WriteString(fmt.Sprintf(" L %.1f %d Z", float64(padding)+xPositions[len(xPositions)-1], chartHeight-padding))

	// Build cloud coverage path (using right Y-axis scale)
	cloudPath := ""
	for i, cloud := range cloudPoints {
		x := float64(padding) + xPositions[i]
		y := float64(chartHeight-padding) - ((cloud-minCloud)/(maxCloud-minCloud))*float64(chartHeight-2*padding)
		if i == 0 {
			cloudPath = fmt.Sprintf("M %.1f %.1f", x, y)
		} else {
			cloudPath += fmt.Sprintf(" L %.1f %.1f", x, y)
		}
	}

	// Add area under production curve
	html.WriteString(fmt.Sprintf(`                    <path class="output-area" d="%s" />
`, areaPath.String()))

	// Add production line
	html.WriteString(fmt.Sprintf(`                    <path class="output-line" d="%s" />
`, productionPath))

	// Add cloud coverage line
	html.WriteString(fmt.Sprintf(`                    <path class="cloud-line" d="%s" />
`, cloudPath))

	// Find minimum production value for highlighting
	minProductionValue := productionPoints[0]
	minProductionIndices := []int{0}
	for i, kw := range productionPoints {
		if kw < minProductionValue {
			minProductionValue = kw
			minProductionIndices = []int{i}
		} else if kw == minProductionValue && i > 0 {
			minProductionIndices = append(minProductionIndices, i)
		}
	}

	// Add production points and labels
	for i, point := range productionPoints {
		x := float64(padding) + xPositions[i]
		y := float64(chartHeight-padding) - ((point-minProduction)/(maxProduction-minProduction))*float64(chartHeight-2*padding)

		// Check if this is a minimum point
		isMinimum := false
		for _, minIdx := range minProductionIndices {
			if i == minIdx {
				isMinimum = true
				break
			}
		}

		// Draw dot with highlighting for minimum
		if isMinimum {
			html.WriteString(fmt.Sprintf(`                    <circle cx="%.1f" cy="%.1f" r="8" fill="#E74C3C" stroke="#C0392B" stroke-width="2" />
`, x, y))
		} else {
			html.WriteString(fmt.Sprintf(`                    <circle class="output-dot" cx="%.1f" cy="%.1f" r="5" />
`, x, y))
		}

		// Draw label only for daylight hours (GHI >= 50)
		if production[i].GHI >= 50.0 {
			html.WriteString(fmt.Sprintf(`                    <text class="output-value" x="%.1f" y="%.1f" text-anchor="middle">%.1f kW</text>
`, x, y-12, point))
		}
	}

	// Add cloud coverage dots and labels
	for i, cloud := range cloudPoints {
		x := float64(padding) + xPositions[i]
		y := float64(chartHeight-padding) - ((cloud-minCloud)/(maxCloud-minCloud))*float64(chartHeight-2*padding)
		html.WriteString(fmt.Sprintf(`                    <circle class="cloud-dot" cx="%.1f" cy="%.1f" r="4" />
`, x, y))

		// Draw label only for daylight hours (GHI >= 50)
		if production[i].GHI >= 50.0 {
			html.WriteString(fmt.Sprintf(`                    <text class="cloud-value" x="%.1f" y="%.1f" text-anchor="middle">%.0f%%</text>
`, x, y+18, cloud))
		}
	}

	// Add x-axis labels (time) - daylight hours only
	html.WriteString(`                    <!-- X-axis labels -->
`)
	for i := 0; i < len(productionPoints); i++ {
		if i < len(production) {
			prod := production[i]
			// Only show time labels during daylight hours (GHI >= 50)
			if prod.GHI >= 50.0 {
				x := float64(padding) + xPositions[i]
				timeStr := prod.Hour.Format("15:04")
				html.WriteString(fmt.Sprintf(`                    <text x="%.1f" y="%.0f" text-anchor="middle" style="font-size: 11px; fill: #2c3e50; font-weight: bold;">%s</text>
`, x, float64(chartHeight-padding+30), timeStr))
			}
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

// getWeatherIcon returns emoji based on conditions
func (a *GmailAdapter) getWeatherIcon(cloudCover int, ghi float64) string {
	if ghi < 100 {
		return "🌙" // Night/dark
	} else if cloudCover >= 80 {
		return "☁️" // Overcast
	} else if cloudCover >= 60 {
		return "⛅" // Mostly cloudy
	} else if cloudCover >= 30 {
		return "🌤️" // Partly cloudy
	}
	return "☀️" // Clear
}

// getConditionText returns descriptive text
func (a *GmailAdapter) getConditionText(cloudCover int, ghi float64) string {
	if ghi < 100 {
		return "Night/Dark"
	} else if cloudCover >= 80 {
		return "Heavily Overcast"
	} else if cloudCover >= 60 {
		return "Mostly Cloudy"
	} else if cloudCover >= 30 {
		return "Partly Cloudy"
	}
	return "Clear Skies"
}

// generateWeatherConditionsTable generates a visual table with weather icons
func (a *GmailAdapter) generateWeatherConditionsTable(hours []domain.SolarProduction) string {
	var html strings.Builder

	// Limit to first 12 hours for readability
	displayHours := hours
	if len(hours) > 12 {
		displayHours = hours[:12]
	}

	html.WriteString(`
        <div style="margin-top: 30px; padding: 30px; background: linear-gradient(to bottom, #FFF, #F8F9FA); border-radius: 12px; border: 2px solid #E0E6ED;">
            <div style="font-size: 20px; font-weight: 700; margin-bottom: 20px; color: #2C3E50;">
                🌦️ Hourly Weather Conditions
            </div>
            <table style="width: 100%; border-collapse: collapse;">
                <thead>
                    <tr style="border-bottom: 2px solid #E0E6ED;">
                        <th style="padding: 12px; text-align: left; font-size: 13px; color: #7F8C8D; font-weight: 700;">Time</th>
                        <th style="padding: 12px; text-align: center; font-size: 13px; color: #7F8C8D; font-weight: 700;">Condition</th>
                        <th style="padding: 12px; text-align: right; font-size: 13px; color: #7F8C8D; font-weight: 700;">Production</th>
                        <th style="padding: 12px; text-align: right; font-size: 13px; color: #7F8C8D; font-weight: 700;">% Capacity</th>
                    </tr>
                </thead>
                <tbody>
`)

	for i, prod := range displayHours {
		// Color code based on production threshold
		var rowBg, borderLeft, textColor, statusIcon string
		if prod.EstimatedOutputKW >= a.alertThresholdKW {
			// Good production - green theme
			if i%2 == 0 {
				rowBg = "#E8F5E9"
			} else {
				rowBg = "#C8E6C9"
			}
			borderLeft = "4px solid #4CAF50"
			textColor = "#2C3E50"
			statusIcon = "✓"
		} else {
			// Low production - red theme
			if i%2 == 0 {
				rowBg = "#FFEBEE"
			} else {
				rowBg = "#FFCDD2"
			}
			borderLeft = "4px solid #F44336"
			textColor = "#C0392B"
			statusIcon = "⚠"
		}

		icon := a.getWeatherIcon(prod.CloudCover, prod.GHI)
		condition := a.getConditionText(prod.CloudCover, prod.GHI)

		html.WriteString(fmt.Sprintf(`
                    <tr style="background: %s; border-left: %s; border-bottom: 1px solid #E0E6ED;">
                        <td style="padding: 12px; font-weight: 600; color: #2C3E50;">%s %s</td>
                        <td style="padding: 12px; text-align: center;">
                            <span style="font-size: 24px;">%s</span>
                            <div style="font-size: 11px; color: #7F8C8D; margin-top: 4px;">%s</div>
                        </td>
                        <td style="padding: 12px; text-align: right; font-weight: 700; color: %s; font-size: 16px;">%.2f kW</td>
                        <td style="padding: 12px; text-align: right; font-weight: 600; color: %s;">%.1f%%</td>
                    </tr>
`, rowBg, borderLeft, prod.Hour.Format("15:04"), statusIcon, icon, condition, textColor, prod.EstimatedOutputKW, textColor, prod.OutputPercentage))
	}

	html.WriteString(`
                </tbody>
            </table>
        </div>
`)

	return html.String()
}

// generateRecoverySection displays when conditions are expected to improve
func (a *GmailAdapter) generateRecoverySection(analysis *domain.AlertAnalysis) string {
	var html strings.Builder

	html.WriteString(`
        <div style="margin-top: 30px; padding: 25px; background: linear-gradient(135deg, #E8F4F8 0%, #D4E9F7 100%); border-radius: 12px; border-left: 4px solid #3498DB; box-shadow: 0 2px 4px rgba(52, 152, 219, 0.1);">
            <div style="font-size: 20px; font-weight: 700; margin-bottom: 15px; color: #2C3E50; display: flex; align-items: center; gap: 10px;">
                <span>🌤️ Recovery Forecast</span>
            </div>
    `)

	if analysis.HasRecovery {
		// Recovery detected within forecast window
		html.WriteString(fmt.Sprintf(`
            <div style="padding: 15px; background: white; border-radius: 8px; margin-bottom: 12px;">
                <div style="font-size: 15px; color: #155724; margin-bottom: 8px;">
                    <strong>✅ Conditions Expected to Improve</strong>
                </div>
                <div style="font-size: 14px; color: #2C3E50; line-height: 1.8;">
                    <div style="padding: 8px 0; border-bottom: 1px solid #E0E6ED;">
                        <strong>Recovery Time:</strong> %s
                    </div>
                    <div style="padding: 8px 0; border-bottom: 1px solid #E0E6ED;">
                        <strong>Low Period Duration:</strong> %d hours (%s to %s)
                    </div>
                    <div style="padding: 8px 0;">
                        <strong>Time Until Recovery:</strong> %d hours from low period start
                    </div>
                </div>
            </div>
            <div style="padding: 12px; background: rgba(52, 152, 219, 0.1); border-radius: 6px; font-size: 13px; color: #2C3E50; line-height: 1.6;">
                💡 <strong>What this means:</strong> Solar production is expected to rise above 2.0 kW at %s,
                approximately %d hours after the low production period begins. Plan your energy usage accordingly.
            </div>
        `,
			analysis.RecoveryHour.Format("15:04 MST"),
			analysis.ConsecutiveHourCount,
			analysis.FirstLowProductionHour.Format("15:04"),
			analysis.LastLowProductionHour.Format("15:04"),
			analysis.HoursUntilRecovery,
			analysis.RecoveryHour.Format("15:04"),
			analysis.HoursUntilRecovery,
		))
	} else {
		// No recovery within 48-hour forecast window
		html.WriteString(fmt.Sprintf(`
            <div style="padding: 15px; background: white; border-radius: 8px; margin-bottom: 12px;">
                <div style="font-size: 15px; color: #C0392B; margin-bottom: 8px;">
                    <strong>⚠️ Extended Low Production Period</strong>
                </div>
                <div style="font-size: 14px; color: #2C3E50; line-height: 1.8;">
                    <div style="padding: 8px 0; border-bottom: 1px solid #E0E6ED;">
                        <strong>Low Period Duration:</strong> %d hours
                    </div>
                    <div style="padding: 8px 0; border-bottom: 1px solid #E0E6ED;">
                        <strong>Period:</strong> %s to %s
                    </div>
                    <div style="padding: 8px 0;">
                        <strong>Recovery:</strong> Not expected within 48-hour forecast window
                    </div>
                </div>
            </div>
            <div style="padding: 12px; background: rgba(192, 57, 43, 0.1); border-radius: 6px; font-size: 13px; color: #2C3E50; line-height: 1.6;">
                💡 <strong>What this means:</strong> Adverse weather conditions may persist beyond the forecast period.
                Consider alternative power arrangements and monitor for updated forecasts.
            </div>
        `,
			analysis.ConsecutiveHourCount,
			analysis.FirstLowProductionHour.Format("15:04"),
			analysis.LastLowProductionHour.Format("15:04"),
		))
	}

	html.WriteString(`
        </div>
    `)

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
            background: linear-gradient(135deg, #28A745 0%, #20C997 50%, #4ADE80 100%);
            color: white;
            padding: 50px 20px;
            text-align: center;
            box-shadow: 0 8px 16px rgba(40, 167, 69, 0.3);
            position: relative;
            overflow: hidden;
        }
        .header::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: radial-gradient(circle at top right, rgba(255,255,255,0.2), transparent);
        }
        .header h1 {
            font-size: 36px;
            margin-bottom: 8px;
            font-weight: 700;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
            position: relative;
            z-index: 1;
        }
        .header .timestamp {
            font-size: 15px;
            opacity: 0.95;
            font-weight: 500;
            position: relative;
            z-index: 1;
        }
        
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

        .success-icon {
            text-align: center;
            padding: 20px;
            font-size: 80px;
            animation: bounce 1s ease-in-out;
        }
        @keyframes bounce {
            0%, 100% { transform: translateY(0); }
            50% { transform: translateY(-20px); }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>☀️ Solar Production Alert Cleared</h1>
            <div class="timestamp">` + time.Now().Format("Monday, January 2 • 15:04 MST") + `</div>
        </div>

        <div class="content">
            <div class="success-icon">✅</div>
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
