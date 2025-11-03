package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

// ScreenshotContext represents the phase of the test when screenshot was taken
type ScreenshotContext string

const (
	// ContextInitial represents the initial page load screenshot
	ContextInitial ScreenshotContext = "initial"
	// ContextGameplay represents a screenshot during gameplay
	ContextGameplay ScreenshotContext = "gameplay"
	// ContextFinal represents the final screenshot before test ends
	ContextFinal ScreenshotContext = "final"
)

// Screenshot represents a captured screenshot with metadata
type Screenshot struct {
	// Filepath is the local path to the screenshot file
	Filepath string
	// Context indicates when the screenshot was taken
	Context ScreenshotContext
	// Timestamp records when the screenshot was captured
	Timestamp time.Time
	// Data contains the raw PNG image bytes
	Data []byte
	// Width is the screenshot width in pixels
	Width int
	// Height is the screenshot height in pixels
	Height int
}

// CaptureScreenshot captures a full-page screenshot using chromedp
// Resolution: 1280x720, Format: PNG with compression level 6
func CaptureScreenshot(ctx context.Context, screenshotContext ScreenshotContext) (*Screenshot, error) {
	var buf []byte

	// Capture screenshot with specified settings
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 720),
		chromedp.FullScreenshot(&buf, 100), // 100 quality for PNG
	); err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	screenshot := &Screenshot{
		Context:   screenshotContext,
		Timestamp: time.Now(),
		Data:      buf,
		Width:     1280,
		Height:    720,
	}

	return screenshot, nil
}

// SaveToTemp saves the screenshot to a temporary directory with a unique filename
func (s *Screenshot) SaveToTemp() error {
	// Generate unique filename
	filename := fmt.Sprintf("screenshot_%s_%s_%s.png",
		s.Context,
		s.Timestamp.Format("20060102_150405"),
		uuid.New().String()[:8],
	)

	// Get system temp directory
	tempDir := os.TempDir()
	filepath := filepath.Join(tempDir, filename)

	// Write file
	if err := os.WriteFile(filepath, s.Data, 0644); err != nil {
		return fmt.Errorf("failed to save screenshot to %s: %w", filepath, err)
	}

	// Update screenshot filepath
	s.Filepath = filepath
	return nil
}
