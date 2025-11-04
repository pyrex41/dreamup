package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dreamup/qa-agent/internal/agent"
	"github.com/dreamup/qa-agent/internal/evaluator"
	"github.com/google/uuid"
)

// Report represents a complete QA test report
type Report struct {
	// ReportID is a unique identifier for this report
	ReportID string `json:"report_id"`
	// GameURL is the URL of the tested game
	GameURL string `json:"game_url"`
	// Timestamp is when the test was conducted
	Timestamp time.Time `json:"timestamp"`
	// Duration is how long the test took
	Duration time.Duration `json:"duration_ms"`
	// Score contains the LLM evaluation results
	Score *evaluator.PlayabilityScore `json:"score,omitempty"`
	// Evidence contains test artifacts
	Evidence *Evidence `json:"evidence"`
	// Summary provides a high-level overview
	Summary *Summary `json:"summary"`
	// Metadata contains additional information
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Evidence contains all test artifacts
type Evidence struct {
	// Screenshots are the captured images
	Screenshots []ScreenshotInfo `json:"screenshots"`
	// VideoURL is the URL to the gameplay video (if recorded)
	VideoURL string `json:"video_url,omitempty"`
	// ConsoleLogs are the browser console logs
	ConsoleLogs []agent.ConsoleLog `json:"console_logs"`
	// LogSummary provides log statistics
	LogSummary LogSummary `json:"log_summary"`
	// DetectedElements are UI elements found
	DetectedElements map[string]string `json:"detected_elements,omitempty"`
	// PerformanceMetrics contains FPS, load time, and accessibility data
	PerformanceMetrics *agent.PerformanceMetrics `json:"performance_metrics,omitempty"`
}

// ScreenshotInfo contains metadata about a screenshot
type ScreenshotInfo struct {
	// Context is when the screenshot was taken
	Context agent.ScreenshotContext `json:"context"`
	// Filepath is the local path
	Filepath string `json:"filepath"`
	// S3URL is the S3 URL (if uploaded)
	S3URL string `json:"s3_url,omitempty"`
	// Timestamp is when it was captured
	Timestamp time.Time `json:"timestamp"`
	// Width in pixels
	Width int `json:"width"`
	// Height in pixels
	Height int `json:"height"`
}

// LogSummary provides console log statistics
type LogSummary struct {
	// Total number of logs
	Total int `json:"total"`
	// Errors count
	Errors int `json:"errors"`
	// Warnings count
	Warnings int `json:"warnings"`
	// Info count
	Info int `json:"info"`
	// Debug count
	Debug int `json:"debug"`
}

// Summary provides a high-level test overview
type Summary struct {
	// Status is the overall test status (passed, failed, error)
	Status string `json:"status"`
	// PassedChecks lists what passed
	PassedChecks []string `json:"passed_checks"`
	// FailedChecks lists what failed
	FailedChecks []string `json:"failed_checks"`
	// CriticalIssues are blocking problems
	CriticalIssues []string `json:"critical_issues"`
}

// ReportBuilder helps construct reports
type ReportBuilder struct {
	gameURL    string
	startTime  time.Time
	screenshots []*agent.Screenshot
	videoURL   string
	logs       []agent.ConsoleLog
	score      *evaluator.PlayabilityScore
	detected   map[string]string
	metadata   map[string]string
	metrics    *agent.PerformanceMetrics
}

// NewReportBuilder creates a new report builder
func NewReportBuilder(gameURL string) *ReportBuilder {
	return &ReportBuilder{
		gameURL:   gameURL,
		startTime: time.Now(),
		detected:  make(map[string]string),
		metadata:  make(map[string]string),
	}
}

// SetScreenshots sets the screenshots for the report
func (rb *ReportBuilder) SetScreenshots(screenshots []*agent.Screenshot) {
	rb.screenshots = screenshots
}

// SetVideoURL sets the video URL for the report
func (rb *ReportBuilder) SetVideoURL(videoURL string) {
	rb.videoURL = videoURL
}

// SetConsoleLogs sets the console logs for the report
func (rb *ReportBuilder) SetConsoleLogs(logs []agent.ConsoleLog) {
	rb.logs = logs
}

// SetScore sets the evaluation score for the report
func (rb *ReportBuilder) SetScore(score *evaluator.PlayabilityScore) {
	rb.score = score
}

// SetDetectedElements sets the detected UI elements
func (rb *ReportBuilder) SetDetectedElements(detected map[string]string) {
	rb.detected = detected
}

// AddMetadata adds a metadata key-value pair
func (rb *ReportBuilder) AddMetadata(key, value string) {
	rb.metadata[key] = value
}

// SetPerformanceMetrics sets the performance metrics for the report
func (rb *ReportBuilder) SetPerformanceMetrics(metrics *agent.PerformanceMetrics) {
	rb.metrics = metrics
}

// Build constructs the final report
func (rb *ReportBuilder) Build() (*Report, error) {
	// Generate report ID
	reportID := uuid.New().String()

	// Calculate duration
	duration := time.Since(rb.startTime)

	// Build screenshot info
	screenshotInfos := make([]ScreenshotInfo, 0, len(rb.screenshots))
	for _, ss := range rb.screenshots {
		screenshotInfos = append(screenshotInfos, ScreenshotInfo{
			Context:   ss.Context,
			Filepath:  ss.Filepath,
			Timestamp: ss.Timestamp,
			Width:     ss.Width,
			Height:    ss.Height,
		})
	}

	// Build log summary
	logSummary := LogSummary{
		Total: len(rb.logs),
	}
	for _, log := range rb.logs {
		switch log.Level {
		case agent.LogLevelError:
			logSummary.Errors++
		case agent.LogLevelWarning:
			logSummary.Warnings++
		case agent.LogLevelInfo:
			logSummary.Info++
		case agent.LogLevelDebug:
			logSummary.Debug++
		}
	}

	// Build evidence
	evidence := &Evidence{
		Screenshots:        screenshotInfos,
		VideoURL:           rb.videoURL,
		ConsoleLogs:        rb.logs,
		LogSummary:         logSummary,
		DetectedElements:   rb.detected,
		PerformanceMetrics: rb.metrics,
	}

	// Build summary
	summary := rb.buildSummary()

	// Create report
	report := &Report{
		ReportID:  reportID,
		GameURL:   rb.gameURL,
		Timestamp: rb.startTime,
		Duration:  duration,
		Score:     rb.score,
		Evidence:  evidence,
		Summary:   summary,
		Metadata:  rb.metadata,
	}

	return report, nil
}

// buildSummary constructs the test summary
func (rb *ReportBuilder) buildSummary() *Summary {
	summary := &Summary{
		PassedChecks:   make([]string, 0),
		FailedChecks:   make([]string, 0),
		CriticalIssues: make([]string, 0),
	}

	// Determine status based on score and logs
	if rb.score != nil {
		if rb.score.LoadsCorrectly {
			summary.PassedChecks = append(summary.PassedChecks, "Game loads successfully")
		} else {
			summary.FailedChecks = append(summary.FailedChecks, "Game failed to load")
			summary.CriticalIssues = append(summary.CriticalIssues, "Game does not load correctly")
		}

		if rb.score.OverallScore >= 70 {
			summary.PassedChecks = append(summary.PassedChecks, "Overall quality acceptable")
		} else if rb.score.OverallScore < 50 {
			summary.FailedChecks = append(summary.FailedChecks, "Overall quality below acceptable threshold")
		}

		if rb.score.InteractivityScore >= 70 {
			summary.PassedChecks = append(summary.PassedChecks, "Game is interactive")
		} else {
			summary.FailedChecks = append(summary.FailedChecks, "Low interactivity")
		}

		if rb.score.ErrorSeverity > 50 {
			summary.CriticalIssues = append(summary.CriticalIssues, "High severity errors detected")
		}

		// Add issues from score
		summary.CriticalIssues = append(summary.CriticalIssues, rb.score.Issues...)
	}

	// Check for console errors
	errorCount := 0
	for _, log := range rb.logs {
		if log.Level == agent.LogLevelError {
			errorCount++
		}
	}

	if errorCount == 0 {
		summary.PassedChecks = append(summary.PassedChecks, "No console errors")
	} else if errorCount > 5 {
		summary.FailedChecks = append(summary.FailedChecks, fmt.Sprintf("%d console errors found", errorCount))
	}

	// Determine overall status
	if len(summary.CriticalIssues) > 0 {
		summary.Status = "failed"
	} else if len(summary.FailedChecks) > 0 {
		summary.Status = "passed_with_warnings"
	} else {
		summary.Status = "passed"
	}

	return summary
}

// SaveToFile saves the report to a JSON file
func (r *Report) SaveToFile(filepath string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report to %s: %w", filepath, err)
	}

	return nil
}

// SaveToTemp saves the report to a temporary file
func (r *Report) SaveToTemp() (string, error) {
	filename := fmt.Sprintf("qa_report_%s_%s.json",
		time.Now().Format("20060102_150405"),
		r.ReportID[:8],
	)

	tempDir := os.TempDir()
	filepath := filepath.Join(tempDir, filename)

	if err := r.SaveToFile(filepath); err != nil {
		return "", err
	}

	return filepath, nil
}
