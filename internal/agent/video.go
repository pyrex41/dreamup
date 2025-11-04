package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

// VideoRecorder captures browser screencast frames and converts them to video
type VideoRecorder struct {
	// Frames stores captured video frames
	Frames [][]byte
	// FrameTimes stores timestamp for each frame
	FrameTimes []time.Time
	// StartTime is when recording started
	StartTime time.Time
	// IsRecording indicates if currently recording
	IsRecording bool
	// mutex for thread-safe operations
	mu sync.Mutex
	// ctx is the browser context
	ctx context.Context
	// ackCtx is the context for frame acknowledgments
	ackCtx context.Context
	// cancel function to stop recording
	cancel context.CancelFunc
	// Format quality (1-100, higher is better)
	Quality int
	// Format frame rate (frames per second)
	FrameRate int
}

// NewVideoRecorder creates a new video recorder instance
func NewVideoRecorder(ctx context.Context) *VideoRecorder {
	return &VideoRecorder{
		Frames:     make([][]byte, 0),
		FrameTimes: make([]time.Time, 0),
		ctx:        ctx,
		Quality:    80,
		FrameRate:  30,
	}
}

// StartRecording begins capturing screencast frames
func (vr *VideoRecorder) StartRecording() error {
	vr.mu.Lock()
	if vr.IsRecording {
		vr.mu.Unlock()
		return fmt.Errorf("recording already in progress")
	}

	vr.IsRecording = true
	vr.StartTime = time.Now()
	vr.Frames = make([][]byte, 0)
	vr.FrameTimes = make([]time.Time, 0)
	vr.mu.Unlock()

	// Set up listener for screencast frames
	// This listener will capture all screencast frame events
	chromedp.ListenTarget(vr.ctx, func(ev interface{}) {
		if frameEvent, ok := ev.(*page.EventScreencastFrame); ok {
			vr.handleFrame(frameEvent)
		}
	})

	// Start screencast with Chrome DevTools Protocol
	// Use ActionFunc to capture the execution context for acknowledgments
	err := chromedp.Run(vr.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		// Store the execution context for frame acknowledgments
		vr.mu.Lock()
		vr.ackCtx = ctx
		vr.mu.Unlock()

		// Start the screencast
		return page.StartScreencast().
			WithFormat(page.ScreencastFormatJpeg).
			WithQuality(int64(vr.Quality)).
			WithEveryNthFrame(1).
			Do(ctx)
	}))

	if err != nil {
		vr.mu.Lock()
		vr.IsRecording = false
		vr.mu.Unlock()
		return fmt.Errorf("failed to start screencast: %w", err)
	}

	return nil
}

// handleFrame processes a screencast frame
func (vr *VideoRecorder) handleFrame(frameEvent *page.EventScreencastFrame) {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	if !vr.IsRecording {
		return
	}

	// Decode base64 frame data
	frameData, err := base64.StdEncoding.DecodeString(frameEvent.Data)
	if err != nil {
		return
	}

	// Store frame and timestamp
	vr.Frames = append(vr.Frames, frameData)
	vr.FrameTimes = append(vr.FrameTimes, time.Now())

	// Acknowledge frame in a goroutine to avoid blocking
	// and to prevent deadlock with mutex
	go func() {
		if vr.ackCtx != nil {
			// Try to acknowledge using the stored context
			if err := page.ScreencastFrameAck(frameEvent.SessionID).Do(vr.ackCtx); err != nil {
				// Silently ignore ack errors - they don't affect frame capture
				// Frame capture will continue even without acknowledgment
			}
		}
	}()
}

// StopRecording stops capturing frames
func (vr *VideoRecorder) StopRecording() error {
	vr.mu.Lock()
	if !vr.IsRecording {
		vr.mu.Unlock()
		return fmt.Errorf("no recording in progress")
	}

	// Set recording to false first to stop accepting new frames
	vr.IsRecording = false
	vr.mu.Unlock()

	// Stop screencast without holding the mutex
	// This allows handleFrame to complete any pending frame processing
	if err := chromedp.Run(vr.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return page.StopScreencast().Do(ctx)
	})); err != nil {
		return fmt.Errorf("failed to stop screencast: %w", err)
	}

	vr.mu.Lock()
	if vr.cancel != nil {
		vr.cancel()
	}
	vr.mu.Unlock()

	return nil
}

// SaveAsMP4 converts captured frames to MP4 video using ffmpeg
func (vr *VideoRecorder) SaveAsMP4(outputPath string) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	if len(vr.Frames) == 0 {
		return fmt.Errorf("no frames captured")
	}

	// Create temporary directory for frames
	tmpDir, err := os.MkdirTemp("", "video_frames_*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write frames to temp files
	for i, frame := range vr.Frames {
		framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%05d.jpg", i))
		if err := os.WriteFile(framePath, frame, 0644); err != nil {
			return fmt.Errorf("failed to write frame %d: %w", i, err)
		}
	}

	// Use ffmpeg to create MP4
	cmd := exec.Command("ffmpeg",
		"-y",                                        // Overwrite output file
		"-framerate", fmt.Sprintf("%d", vr.FrameRate), // Input frame rate
		"-i", filepath.Join(tmpDir, "frame_%05d.jpg"), // Input pattern
		"-c:v", "libx264",    // H.264 codec
		"-preset", "fast",    // Encoding speed preset
		"-pix_fmt", "yuv420p", // Pixel format for compatibility
		"-crf", "23",         // Quality (lower is better, 23 is good)
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// SaveToTemp saves the video to a temporary file
func (vr *VideoRecorder) SaveToTemp() (string, error) {
	filename := fmt.Sprintf("gameplay_%s_%s.mp4",
		time.Now().Format("20060102_150405"),
		uuid.New().String()[:8],
	)

	tempDir := os.TempDir()
	filepath := filepath.Join(tempDir, filename)

	if err := vr.SaveAsMP4(filepath); err != nil {
		return "", err
	}

	return filepath, nil
}

// GetDuration returns the recording duration
func (vr *VideoRecorder) GetDuration() time.Duration {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	if len(vr.FrameTimes) < 2 {
		return 0
	}

	return vr.FrameTimes[len(vr.FrameTimes)-1].Sub(vr.FrameTimes[0])
}

// GetFrameCount returns the number of frames captured
func (vr *VideoRecorder) GetFrameCount() int {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	return len(vr.Frames)
}
