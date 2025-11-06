package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/runtime"
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

// getMediaDir returns the persistent media directory, creating it if needed
func getMediaDir() (string, error) {
	// Use ./data/media for persistent storage (not /tmp which is ephemeral)
	mediaDir := filepath.Join(".", "data", "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create media directory: %w", err)
	}
	return mediaDir, nil
}

// SaveToTemp saves the screenshot to a persistent directory with a unique filename
func (s *Screenshot) SaveToTemp() error {
	// Generate unique filename
	filename := fmt.Sprintf("screenshot_%s_%s_%s.png",
		s.Context,
		s.Timestamp.Format("20060102_150405"),
		uuid.New().String()[:8],
	)

	// Get persistent media directory
	mediaDir, err := getMediaDir()
	if err != nil {
		return err
	}
	filepath := filepath.Join(mediaDir, filename)

	// Write file
	if err := os.WriteFile(filepath, s.Data, 0644); err != nil {
		return fmt.Errorf("failed to save screenshot to %s: %w", filepath, err)
	}

	// Update screenshot filepath - store only the filename for HTTP access via /media/ endpoint
	// Frontend will access as /media/filename instead of data/media/filename
	s.Filepath = filename
	return nil
}

// Hash computes a SHA256 hash of the screenshot data for change detection
// Returns a hex-encoded string of the hash
func (s *Screenshot) Hash() string {
	hash := sha256.Sum256(s.Data)
	return hex.EncodeToString(hash[:])
}

// LogLevel represents the severity level of a console log
type LogLevel string

const (
	// LogLevelLog represents console.log messages
	LogLevelLog LogLevel = "log"
	// LogLevelWarning represents console.warn messages
	LogLevelWarning LogLevel = "warning"
	// LogLevelError represents console.error messages
	LogLevelError LogLevel = "error"
	// LogLevelInfo represents console.info messages
	LogLevelInfo LogLevel = "info"
	// LogLevelDebug represents console.debug messages
	LogLevelDebug LogLevel = "debug"
)

// ConsoleLog represents a single browser console log entry
type ConsoleLog struct {
	// Level is the severity level of the log
	Level LogLevel
	// Message is the log message text
	Message string
	// Timestamp records when the log occurred
	Timestamp time.Time
	// Source is the file/line where the log originated (if available)
	Source string
	// Args contains additional arguments passed to the console method
	Args []interface{}
}

// ConsoleLogger captures browser console logs during test execution
type ConsoleLogger struct {
	// Logs is the collection of captured console logs
	Logs []ConsoleLog
	// Filter determines which log levels to capture (nil = capture all)
	Filter map[LogLevel]bool
}

// NewConsoleLogger creates a new console logger with optional level filtering
func NewConsoleLogger(filterLevels ...LogLevel) *ConsoleLogger {
	logger := &ConsoleLogger{
		Logs: make([]ConsoleLog, 0),
	}

	// If filter levels provided, create filter map
	if len(filterLevels) > 0 {
		logger.Filter = make(map[LogLevel]bool)
		for _, level := range filterLevels {
			logger.Filter[level] = true
		}
	}

	return logger
}

// StartCapture sets up console log event listeners in the browser context
func (cl *ConsoleLogger) StartCapture(ctx context.Context) error {
	// Enable console log events
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			cl.handleConsoleEvent(ev)
		}
	})

	// Enable Runtime domain to receive console events
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return runtime.Enable().Do(ctx)
	})); err != nil {
		return fmt.Errorf("failed to enable runtime events: %w", err)
	}

	return nil
}

// handleConsoleEvent processes a console API event
func (cl *ConsoleLogger) handleConsoleEvent(ev *runtime.EventConsoleAPICalled) {
	level := LogLevel(ev.Type.String())

	// Apply filter if configured
	if cl.Filter != nil {
		if !cl.Filter[level] {
			return
		}
	}

	// Extract message from first argument
	message := ""
	args := make([]interface{}, 0)

	for i, arg := range ev.Args {
		if arg.Value != nil {
			var value interface{}
			if err := json.Unmarshal(arg.Value, &value); err == nil {
				if i == 0 {
					message = fmt.Sprintf("%v", value)
				} else {
					args = append(args, value)
				}
			}
		}
	}

	// Get source location if available
	source := ""
	if ev.StackTrace != nil && len(ev.StackTrace.CallFrames) > 0 {
		frame := ev.StackTrace.CallFrames[0]
		source = fmt.Sprintf("%s:%d:%d", frame.URL, frame.LineNumber, frame.ColumnNumber)
	}

	log := ConsoleLog{
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
		Source:    source,
		Args:      args,
	}

	cl.Logs = append(cl.Logs, log)
}

// GetLogs returns all captured logs
func (cl *ConsoleLogger) GetLogs() []ConsoleLog {
	return cl.Logs
}

// GetLogsByLevel returns logs filtered by specific level
func (cl *ConsoleLogger) GetLogsByLevel(level LogLevel) []ConsoleLog {
	filtered := make([]ConsoleLog, 0)
	for _, log := range cl.Logs {
		if log.Level == level {
			filtered = append(filtered, log)
		}
	}
	return filtered
}

// SaveToFile saves console logs to a JSON file
func (cl *ConsoleLogger) SaveToFile(filepath string) error {
	data, err := json.MarshalIndent(cl.Logs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal console logs: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write console logs to %s: %w", filepath, err)
	}

	return nil
}

// SaveToTemp saves console logs to a temporary file
func (cl *ConsoleLogger) SaveToTemp() (string, error) {
	filename := fmt.Sprintf("console_logs_%s_%s.json",
		time.Now().Format("20060102_150405"),
		uuid.New().String()[:8],
	)

	tempDir := os.TempDir()
	filepath := filepath.Join(tempDir, filename)

	if err := cl.SaveToFile(filepath); err != nil {
		return "", err
	}

	return filepath, nil
}
