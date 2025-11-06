package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/dreamup/qa-agent/internal/agent"
	"github.com/dreamup/qa-agent/internal/evaluator"
	"github.com/dreamup/qa-agent/internal/reporter"
)

// LambdaEvent represents the input event for Lambda
type LambdaEvent struct {
	// GameURL is the URL to test
	GameURL string `json:"game_url"`
	// Timeout in seconds (default: 280 for 5min Lambda - buffer)
	Timeout int `json:"timeout,omitempty"`
	// UploadToS3 determines if artifacts should be uploaded
	UploadToS3 bool `json:"upload_to_s3"`
	// BucketName for S3 uploads (optional, defaults to env var)
	BucketName string `json:"bucket_name,omitempty"`
	// Metadata for the test
	Metadata map[string]string `json:"metadata,omitempty"`
}

// LambdaResponse represents the Lambda function output
type LambdaResponse struct {
	// Success indicates if the test completed
	Success bool `json:"success"`
	// ReportID is the unique report identifier
	ReportID string `json:"report_id,omitempty"`
	// ReportURL is the S3 URL (if uploaded)
	ReportURL string `json:"report_url,omitempty"`
	// Status is the test outcome (passed, failed, error)
	Status string `json:"status,omitempty"`
	// Error message if failed
	Error string `json:"error,omitempty"`
	// Summary provides brief results
	Summary *reporter.Summary `json:"summary,omitempty"`
	// Duration in seconds
	Duration float64 `json:"duration_seconds,omitempty"`
}

// HandleRequest is the Lambda handler function
func HandleRequest(ctx context.Context, event LambdaEvent) (LambdaResponse, error) {
	startTime := time.Now()

	// Validate input
	if event.GameURL == "" {
		return LambdaResponse{
			Success: false,
			Error:   "game_url is required",
		}, fmt.Errorf("missing game_url")
	}

	// Validate URL format and scheme
	parsedURL, err := url.ParseRequestURI(event.GameURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return LambdaResponse{
			Success: false,
			Error:   "game_url must be a valid http or https URL",
		}, fmt.Errorf("invalid game_url: %s", event.GameURL)
	}

	// Set default timeout (increased buffer to 60s for safe cleanup)
	if event.Timeout == 0 {
		event.Timeout = 240 // 5 min Lambda - 60s buffer (was 280 - 20s)
	}

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, time.Duration(event.Timeout)*time.Second)
	defer cancel()

	// Initialize report builder
	reportBuilder := reporter.NewReportBuilder(event.GameURL)
	reportBuilder.AddMetadata("lambda_execution", "true")
	reportBuilder.AddMetadata("lambda_region", os.Getenv("AWS_REGION"))

	// Add custom metadata
	for k, v := range event.Metadata {
		reportBuilder.AddMetadata(k, v)
	}

	// Run test with retry logic
	var report *reporter.Report
	var screenshots []*agent.Screenshot
	var logFilepath string

	err = agent.WithRetry(testCtx, func() error {
		// Create browser manager (always headless in lambda)
		bm, err := agent.NewBrowserManager(true)
		if err != nil {
			return agent.NewBrowserError("failed to create browser", err)
		}
		defer bm.Close()

		// Start console logger
		consoleLogger := agent.NewConsoleLogger()
		if err := consoleLogger.StartCapture(bm.GetContext()); err != nil {
			// Non-fatal, continue without console logs
			fmt.Fprintf(os.Stderr, "Warning: console logger failed: %v\n", err)
		}

		// Load game
		if err := bm.LoadGame(event.GameURL); err != nil {
			return agent.NewNetworkError("failed to load game", err)
		}

		// Capture initial screenshot
		initialScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextInitial)
		if err != nil {
			return agent.NewBrowserError("failed to capture initial screenshot", err)
		}
		if err := initialScreenshot.SaveToTemp(); err != nil {
			return agent.NewStorageError("failed to save screenshot", err)
		}

		// Wait for gameplay
		time.Sleep(5 * time.Second)

		// Capture final screenshot
		finalScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextFinal)
		if err != nil {
			return agent.NewBrowserError("failed to capture final screenshot", err)
		}
		if err := finalScreenshot.SaveToTemp(); err != nil {
			return agent.NewStorageError("failed to save screenshot", err)
		}

		screenshots = []*agent.Screenshot{initialScreenshot, finalScreenshot}

		// Save console logs
		logPath, err := consoleLogger.SaveToTemp()
		if err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: failed to save logs: %v\n", err)
		} else {
			logFilepath = logPath
		}

		// Evaluate with LLM (with retry)
		logs := consoleLogger.GetLogs()
		gameEval, err := evaluator.NewGameEvaluator("")
		if err != nil {
			// Non-fatal - continue without evaluation
			fmt.Fprintf(os.Stderr, "Warning: LLM evaluator unavailable: %v\n", err)
		} else {
			score, err := gameEval.EvaluateGame(testCtx, screenshots, logs)
			if err != nil {
				// Log but don't fail
				fmt.Fprintf(os.Stderr, "Warning: LLM evaluation failed: %v\n", err)
			} else {
				reportBuilder.SetScore(score)
			}
		}

		// Build report
		reportBuilder.SetScreenshots(screenshots)
		reportBuilder.SetConsoleLogs(logs)

		builtReport, err := reportBuilder.Build()
		if err != nil {
			return fmt.Errorf("failed to build report: %w", err)
		}

		report = builtReport
		return nil
	})

	duration := time.Since(startTime)

	// Handle test failure
	if err != nil {
		return LambdaResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Seconds(),
		}, nil // Don't return error to Lambda - include in response
	}

	// Save report locally
	reportPath, err := report.SaveToTemp()
	if err != nil {
		return LambdaResponse{
			Success:  false,
			Error:    fmt.Sprintf("failed to save report: %v", err),
			Duration: duration.Seconds(),
		}, nil
	}

	response := LambdaResponse{
		Success:  true,
		ReportID: report.ReportID,
		Status:   report.Summary.Status,
		Summary:  report.Summary,
		Duration: duration.Seconds(),
	}

	// Upload to S3 if requested
	if event.UploadToS3 {
		bucketName := event.BucketName
		if bucketName == "" {
			bucketName = os.Getenv("S3_BUCKET_NAME")
		}

		uploader, err := reporter.NewS3Uploader(bucketName, "")
		if err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: S3 upload skipped: %v\n", err)
		} else {
			err = uploader.UploadReportWithArtifacts(testCtx, report, screenshots, logFilepath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: S3 upload failed: %v\n", err)
			} else {
				response.ReportURL = uploader.GetReportURL(report.ReportID)
			}
		}
	}

	// Clean up temp files
	os.Remove(reportPath)
	if logFilepath != "" {
		os.Remove(logFilepath)
	}
	for _, ss := range screenshots {
		if ss.Filepath != "" {
			os.Remove(ss.Filepath)
		}
	}

	return response, nil
}

func main() {
	lambda.Start(HandleRequest)
}
