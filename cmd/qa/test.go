package main

import (
	"fmt"
	"time"

	"github.com/dreamup/qa-agent/internal/agent"
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

	// Ensure output directory exists
	if err := EnsureOutputDir(outputDir); err != nil {
		return err
	}

	fmt.Println("🌐 Starting browser...")
	// Create browser manager
	bm, err := agent.NewBrowserManager()
	if err != nil {
		return fmt.Errorf("failed to create browser manager: %w", err)
	}
	defer bm.Close()

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

	// Wait a bit for gameplay
	fmt.Println("⏳ Waiting for gameplay...")
	time.Sleep(5 * time.Second)

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

	fmt.Println("\n✅ Test completed successfully!")
	fmt.Printf("📁 Screenshots saved to temp directory\n")

	return nil
}
