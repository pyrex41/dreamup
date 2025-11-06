package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/dreamup/qa-agent/internal/agent"
	"github.com/dreamup/qa-agent/internal/evaluator"
	"github.com/dreamup/qa-agent/internal/reporter"
	"github.com/spf13/cobra"
)

var (
	// Test command flags
	testURL       string
	outputDir     string
	headless      bool
	maxDuration   int
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run QA test on a game URL",
	Long: `Execute a QA test session on a specified game URL.
The agent will launch a browser, navigate to the game, interact with it,
capture screenshots, and generate a test report.`,
	RunE: runTest,
}

func init() {
	// Define flags for test command
	testCmd.Flags().StringVarP(&testURL, "url", "u", "", "Game URL to test (required)")
	testCmd.Flags().StringVarP(&outputDir, "output", "o", "./qa-results", "Output directory for test results")
	testCmd.Flags().BoolVar(&headless, "headless", true, "Run browser in headless mode")
	testCmd.Flags().IntVarP(&maxDuration, "max-duration", "d", 300, "Maximum test duration in seconds")

	// Mark required flags
	testCmd.MarkFlagRequired("url")
}

func runTest(cmd *cobra.Command, args []string) error {
	fmt.Printf("🚀 DreamUp QA Agent v%s\n", version)
	fmt.Printf("📋 Test Configuration:\n")
	fmt.Printf("   URL: %s\n", testURL)
	fmt.Printf("   Output Directory: %s\n", outputDir)
	fmt.Printf("   Headless Mode: %v\n", headless)
	fmt.Printf("   Max Duration: %d seconds\n", maxDuration)
	fmt.Println()

	// Initialize report builder
	reportBuilder := reporter.NewReportBuilder(testURL)
	reportBuilder.AddMetadata("agent_version", version)
	reportBuilder.AddMetadata("headless", fmt.Sprintf("%v", headless))

	// Ensure output directory exists
	if err := EnsureOutputDir(outputDir); err != nil {
		return err
	}

	fmt.Println("🌐 Starting browser...")
	// Create browser manager
	bm, err := agent.NewBrowserManager(headless)
	if err != nil {
		return fmt.Errorf("failed to create browser manager: %w", err)
	}
	defer bm.Close()

	fmt.Println("📝 Starting console log capture...")
	// Create and start console logger
	consoleLogger := agent.NewConsoleLogger()
	if err := consoleLogger.StartCapture(bm.GetContext()); err != nil {
		return fmt.Errorf("failed to start console logger: %w", err)
	}

	fmt.Printf("📍 Navigating to %s...\n", testURL)
	// Load the game URL
	if err := bm.LoadGame(testURL); err != nil {
		return fmt.Errorf("failed to load game: %w", err)
	}

	fmt.Println("📸 Capturing initial screenshot...")
	// Capture initial screenshot
	initialScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextInitial)
	if err != nil {
		return fmt.Errorf("failed to capture initial screenshot: %w", err)
	}

	if err := initialScreenshot.SaveToTemp(); err != nil {
		return fmt.Errorf("failed to save initial screenshot: %w", err)
	}
	fmt.Printf("   Saved: %s\n", initialScreenshot.Filepath)

	// Wait for page resources to load (reduced from 4s to 2s)
	fmt.Println("⏳ Waiting for page to load...")
	time.Sleep(2 * time.Second)

	// Handle cookie consent if present
	fmt.Println("🍪 Checking for cookie consent...")
	detector := agent.NewUIDetector(bm.GetContext())
	var uiWarnings []string
	if clicked, err := detector.AcceptCookieConsent(); err != nil {
		warning := fmt.Sprintf("Cookie consent check failed: %v", err)
		uiWarnings = append(uiWarnings, warning)
		fmt.Printf("   ⚠️  Warning: %s\n", warning)
	} else if clicked {
		fmt.Println("   ✅ Cookie consent accepted")
		// Brief wait for dialog animation (reduced from 1s to 500ms)
		time.Sleep(500 * time.Millisecond)
	} else {
		fmt.Println("   No cookie consent dialog detected")
	}

	// Try to find and click the start/play button
	fmt.Println("🎮 Looking for start button...")
	if clicked, err := detector.ClickStartButton(); err != nil {
		warning := fmt.Sprintf("Start button check failed: %v", err)
		uiWarnings = append(uiWarnings, warning)
		fmt.Printf("   ⚠️  Warning: %s\n", warning)
	} else if clicked {
		fmt.Println("   ✅ Game started!")
		// Brief wait for game initialization (reduced from 1s to 500ms)
		time.Sleep(500 * time.Millisecond)
	} else {
		fmt.Println("   No start button detected, game may auto-start")
	}

	// Wait for game to render initial state (reduced from 3s to 2s)
	fmt.Println("⏳ Waiting for gameplay...")
	time.Sleep(2 * time.Second)

	// Simulate some gameplay interactions
	fmt.Println("🕹️  Simulating gameplay...")
	gameplayActions := []struct {
		key  string
		desc string
	}{
		{"ArrowUp", "up"},
		{"ArrowDown", "down"},
		{"Space", "space"},
		{"ArrowLeft", "left"},
		{"ArrowRight", "right"},
	}

	for _, action := range gameplayActions {
		chromedp.Run(bm.GetContext(),
			chromedp.KeyEvent(action.key),
		)
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("   Game interactions completed")
	// Brief wait for game state to settle (reduced from 1s to 500ms)
	time.Sleep(500 * time.Millisecond)

	fmt.Println("📸 Capturing final screenshot...")
	// Capture final screenshot
	finalScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextFinal)
	if err != nil {
		return fmt.Errorf("failed to capture final screenshot: %w", err)
	}

	if err := finalScreenshot.SaveToTemp(); err != nil {
		return fmt.Errorf("failed to save final screenshot: %w", err)
	}
	fmt.Printf("   Saved: %s\n", finalScreenshot.Filepath)

	// Save console logs
	fmt.Println("💾 Saving console logs...")
	logFilepath, err := consoleLogger.SaveToTemp()
	if err != nil {
		return fmt.Errorf("failed to save console logs: %w", err)
	}
	fmt.Printf("   Saved: %s\n", logFilepath)

	// Display log summary
	logs := consoleLogger.GetLogs()
	errors := consoleLogger.GetLogsByLevel(agent.LogLevelError)
	warnings := consoleLogger.GetLogsByLevel(agent.LogLevelWarning)

	fmt.Printf("\n📊 Console Log Summary:\n")
	fmt.Printf("   Total: %d logs\n", len(logs))
	fmt.Printf("   Errors: %d\n", len(errors))
	fmt.Printf("   Warnings: %d\n", len(warnings))

	// Evaluate game playability using LLM
	fmt.Println("\n🤖 Evaluating game playability with AI...")
	gameEval, err := evaluator.NewGameEvaluator("")
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not initialize evaluator: %v\n", err)
		fmt.Println("   Skipping AI evaluation (set OPENAI_API_KEY to enable)")
	} else {
		screenshots := []*agent.Screenshot{initialScreenshot, finalScreenshot}
		score, err := gameEval.EvaluateGame(context.Background(), screenshots, logs)
		if err != nil {
			fmt.Printf("⚠️  Warning: AI evaluation failed: %v\n", err)
		} else {
			// Display evaluation results
			fmt.Printf("\n🎯 AI Evaluation Results:\n")
			fmt.Printf("   Overall Score: %d/100\n", score.OverallScore)
			fmt.Printf("   Loads Correctly: %v\n", score.LoadsCorrectly)
			fmt.Printf("   Interactivity: %d/100\n", score.InteractivityScore)
			fmt.Printf("   Visual Quality: %d/100\n", score.VisualQuality)
			fmt.Printf("   Error Severity: %d/100\n", score.ErrorSeverity)

			if len(score.Issues) > 0 {
				fmt.Printf("\n   Issues Found:\n")
				for _, issue := range score.Issues {
					fmt.Printf("   - %s\n", issue)
				}
			}

			if len(score.Recommendations) > 0 {
				fmt.Printf("\n   Recommendations:\n")
				for _, rec := range score.Recommendations {
					fmt.Printf("   - %s\n", rec)
				}
			}

			fmt.Printf("\n   Reasoning: %s\n", score.Reasoning)

			// Set score in report builder
			reportBuilder.SetScore(score)
		}
	}

	// Build report
	fmt.Println("\n📊 Generating test report...")
	screenshots := []*agent.Screenshot{initialScreenshot, finalScreenshot}
	reportBuilder.SetScreenshots(screenshots)
	reportBuilder.SetConsoleLogs(logs)

	// Add UI warnings to report metadata
	for i, warning := range uiWarnings {
		reportBuilder.AddMetadata(fmt.Sprintf("ui_warning_%d", i+1), warning)
	}
	if len(uiWarnings) > 0 {
		reportBuilder.AddMetadata("ui_warning_count", fmt.Sprintf("%d", len(uiWarnings)))
	}

	report, err := reportBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build report: %w", err)
	}

	// Save report locally
	reportPath, err := report.SaveToTemp()
	if err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}
	fmt.Printf("   Report saved: %s\n", reportPath)

	// Upload to S3 (optional)
	s3Uploader, err := reporter.NewS3Uploader("", "")
	if err != nil {
		fmt.Printf("   ⚠️  S3 upload skipped (configure AWS credentials to enable): %v\n", err)
	} else {
		fmt.Println("   Uploading artifacts to S3...")
		err = s3Uploader.UploadReportWithArtifacts(context.Background(), report, screenshots, logFilepath)
		if err != nil {
			fmt.Printf("   ⚠️  S3 upload failed: %v\n", err)
		} else {
			s3URL := s3Uploader.GetReportURL(report.ReportID)
			fmt.Printf("   ✅ Report uploaded: %s\n", s3URL)
		}
	}

	// Display summary
	fmt.Printf("\n📋 Test Summary:\n")
	fmt.Printf("   Status: %s\n", report.Summary.Status)
	fmt.Printf("   Duration: %.2f seconds\n", report.Duration.Seconds())
	if len(report.Summary.PassedChecks) > 0 {
		fmt.Printf("   ✅ Passed: %d checks\n", len(report.Summary.PassedChecks))
	}
	if len(report.Summary.FailedChecks) > 0 {
		fmt.Printf("   ⚠️  Failed: %d checks\n", len(report.Summary.FailedChecks))
	}
	if len(report.Summary.CriticalIssues) > 0 {
		fmt.Printf("   ❌ Critical: %d issues\n", len(report.Summary.CriticalIssues))
	}

	fmt.Println("\n✅ Test completed successfully!")
	fmt.Printf("📁 Report ID: %s\n", report.ReportID)

	return nil
}
