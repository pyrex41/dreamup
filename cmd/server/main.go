package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dreamup/qa-agent/internal/agent"
	"github.com/dreamup/qa-agent/internal/db"
	"github.com/dreamup/qa-agent/internal/evaluator"
	"github.com/dreamup/qa-agent/internal/reporter"
	"github.com/google/uuid"
)

const (
	version = "0.1.0"
)

// TestRequest represents a test submission
type TestRequest struct {
	URL         string `json:"url"`
	MaxDuration int    `json:"maxDuration,omitempty"`
	Headless    bool   `json:"headless"`
}

// TestResponse represents the test submission response
type TestResponse struct {
	TestID string `json:"testId"`
	Status string `json:"status"`
}

// BatchTestRequest represents a batch test submission (max 10 URLs)
type BatchTestRequest struct {
	URLs        []string `json:"urls"`
	MaxDuration int      `json:"maxDuration,omitempty"`
	Headless    bool     `json:"headless"`
}

// BatchTestResponse represents the batch test submission response
type BatchTestResponse struct {
	BatchID string   `json:"batchId"`
	TestIDs []string `json:"testIds"`
	Status  string   `json:"status"`
}

// BatchTestStatus represents the status of a batch test
type BatchTestStatus struct {
	BatchID        string       `json:"batchId"`
	Status         string       `json:"status"`
	Tests          []TestStatus `json:"tests"`
	TotalTests     int          `json:"totalTests"`
	CompletedTests int          `json:"completedTests"`
	FailedTests    int          `json:"failedTests"`
	RunningTests   int          `json:"runningTests"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

// BatchJob represents a batch of test jobs
type BatchJob struct {
	ID        string
	TestIDs   []string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TestStatus represents the current status of a test
type TestStatus struct {
	TestID    string    `json:"testId"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TestJob represents a running test
type TestJob struct {
	ID        string
	Request   TestRequest
	Status    string
	Progress  int
	Message   string
	Report    *reporter.Report
	CreatedAt time.Time
	UpdatedAt time.Time
	Error     error
	ctx       context.Context
	cancel    context.CancelFunc
}

// Server manages the API and test execution
type Server struct {
	jobs           map[string]*TestJob
	batchJobs      map[string]*BatchJob
	mu             sync.RWMutex
	port           string
	apiKey         string
	testSemaphore  chan struct{} // Limits concurrent tests
	maxConcurrent  int
	db             *db.Database
}

func NewServer(port, apiKey string) *Server {
	maxConcurrent := 20 // Maximum 20 concurrent tests
	return &Server{
		jobs:          make(map[string]*TestJob),
		batchJobs:     make(map[string]*BatchJob),
		port:          port,
		apiKey:        apiKey,
		testSemaphore: make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// CORS middleware
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// Config endpoint - returns server configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	forceHeadless := os.Getenv("FORCE_HEADLESS") == "true"
	json.NewEncoder(w).Encode(map[string]interface{}{
		"forceHeadless": forceHeadless,
	})
}

// Health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"version": version,
		"time":    time.Now(),
	})
}

// Submit a new test
func (s *Server) handleTestSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.MaxDuration == 0 {
		req.MaxDuration = 60
	}

	// Create test job
	testID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	job := &TestJob{
		ID:        testID,
		Request:   req,
		Status:    "pending",
		Progress:  0,
		Message:   "Test queued",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	s.mu.Lock()
	s.jobs[testID] = job
	s.mu.Unlock()

	// Persist test to database
	if err := s.db.CreateTest(testID, req.URL, "pending"); err != nil {
		log.Printf("Warning: Failed to persist test to database: %v", err)
		// Continue anyway - test will run in memory
	}

	// Start test execution in background
	go s.executeTest(job)

	// Return test ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TestResponse{
		TestID: testID,
		Status: "pending",
	})
}

// Get test status
func (s *Server) handleTestStatus(w http.ResponseWriter, r *http.Request) {
	testID := r.URL.Path[len("/api/tests/"):]
	if testID == "" {
		http.Error(w, "Test ID required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	job, exists := s.jobs[testID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Test not found", http.StatusNotFound)
		return
	}

	status := TestStatus{
		TestID:    job.ID,
		Status:    job.Status,
		Progress:  job.Progress,
		Message:   job.Message,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Get test report
func (s *Server) handleTestReport(w http.ResponseWriter, r *http.Request) {
	testID := r.URL.Path[len("/api/reports/"):]
	if testID == "" {
		http.Error(w, "Test ID required", http.StatusBadRequest)
		return
	}

	// First check in-memory jobs (for active tests)
	s.mu.RLock()
	job, exists := s.jobs[testID]
	s.mu.RUnlock()

	if exists && job.Report != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job.Report)
		return
	}

	// If not in memory, check database
	dbTest, err := s.db.GetTest(testID)
	if err != nil {
		http.Error(w, "Test not found", http.StatusNotFound)
		return
	}

	// Parse the report data JSON
	if dbTest.ReportData == "" {
		http.Error(w, "Report not available", http.StatusNotFound)
		return
	}

	var report reporter.Report
	if err := json.Unmarshal([]byte(dbTest.ReportData), &report); err != nil {
		log.Printf("Failed to parse report for test %s: %v", testID, err)
		http.Error(w, "Failed to parse report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// Serve screenshot files
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	// Extract filename from path: /api/screenshots/{filename}
	filename := r.URL.Path[len("/api/screenshots/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Security: prevent directory traversal and only allow screenshot files
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || !strings.HasPrefix(filename, "screenshot_") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Read screenshot from temp directory
	tmpDir := os.TempDir()
	filepath := tmpDir + "/" + filename

	data, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Failed to read screenshot %s: %v", filepath, err)
		http.Error(w, "Screenshot not found", http.StatusNotFound)
		return
	}

	// Determine content type based on file extension
	contentType := "image/png"
	if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
		contentType = "image/jpeg"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

// Serve video files
func (s *Server) handleVideo(w http.ResponseWriter, r *http.Request) {
	// Extract filename from path: /api/videos/{filename}
	filename := r.URL.Path[len("/api/videos/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Security: prevent directory traversal and only allow video files
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || !strings.HasPrefix(filename, "gameplay_") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Read video from temp directory
	tmpDir := os.TempDir()
	filePath := filepath.Join(tmpDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read video %s: %v", filePath, err)
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	// Set content type for MP4 video
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

// ReportSummary represents a test summary for the history page
type ReportSummary struct {
	ReportID     string  `json:"reportId"`
	GameURL      string  `json:"gameUrl"`
	Timestamp    string  `json:"timestamp"`
	Status       string  `json:"status"`
	OverallScore *int    `json:"overallScore"`
	Duration     int     `json:"duration"`
}

// List all tests
func (s *Server) handleTestList(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "all"
	}

	// Query database for tests (default: 100 most recent)
	dbTests, err := s.db.ListTests(statusFilter, 100, 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert database records to ReportSummary for API response
	summaries := make([]ReportSummary, 0, len(dbTests))
	for _, dbTest := range dbTests {
		var score *int
		if dbTest.Score > 0 {
			score = &dbTest.Score
		}

		summaries = append(summaries, ReportSummary{
			ReportID:     dbTest.ID,
			GameURL:      dbTest.GameURL,
			Timestamp:    dbTest.CreatedAt.Format(time.RFC3339),
			Status:       dbTest.Status,
			OverallScore: score,
			Duration:     dbTest.Duration,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// Submit a batch test (up to 10 URLs)
func (s *Server) handleBatchTestSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BatchTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.URLs) == 0 {
		http.Error(w, "At least one URL is required", http.StatusBadRequest)
		return
	}

	if len(req.URLs) > 10 {
		http.Error(w, "Maximum 10 URLs allowed per batch", http.StatusBadRequest)
		return
	}

	// Validate each URL
	for _, url := range req.URLs {
		if url == "" {
			http.Error(w, "All URLs must be non-empty", http.StatusBadRequest)
			return
		}
	}

	// Set defaults
	if req.MaxDuration == 0 {
		req.MaxDuration = 60
	}

	// Create batch ID
	batchID := uuid.New().String()
	testIDs := make([]string, 0, len(req.URLs))

	// Create individual test jobs for each URL
	for _, url := range req.URLs {
		testID := uuid.New().String()
		ctx, cancel := context.WithCancel(context.Background())

		job := &TestJob{
			ID: testID,
			Request: TestRequest{
				URL:         url,
				MaxDuration: req.MaxDuration,
				Headless:    req.Headless,
			},
			Status:    "pending",
			Progress:  0,
			Message:   "Test queued in batch",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ctx:       ctx,
			cancel:    cancel,
		}

		s.mu.Lock()
		s.jobs[testID] = job
		s.mu.Unlock()

		// Persist test to database
		if err := s.db.CreateTest(testID, url, "pending"); err != nil {
			log.Printf("Warning: Failed to persist batch test to database: %v", err)
			// Continue anyway - test will run in memory
		}

		testIDs = append(testIDs, testID)

		// Start test execution in background
		go s.executeTest(job)
	}

	// Create batch job
	batchJob := &BatchJob{
		ID:        batchID,
		TestIDs:   testIDs,
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.mu.Lock()
	s.batchJobs[batchID] = batchJob
	s.mu.Unlock()

	// Start batch status monitor
	go s.monitorBatchStatus(batchID)

	// Return batch ID and test IDs
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BatchTestResponse{
		BatchID: batchID,
		TestIDs: testIDs,
		Status:  "running",
	})
}

// Get batch test status
func (s *Server) handleBatchTestStatus(w http.ResponseWriter, r *http.Request) {
	batchID := r.URL.Path[len("/api/batch-tests/"):]
	if batchID == "" {
		http.Error(w, "Batch ID required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	batchJob, exists := s.batchJobs[batchID]
	if !exists {
		s.mu.RUnlock()
		http.Error(w, "Batch not found", http.StatusNotFound)
		return
	}

	// Collect status of all tests in the batch and calculate statistics
	tests := make([]TestStatus, 0, len(batchJob.TestIDs))
	completedCount := 0
	failedCount := 0
	runningCount := 0

	for _, testID := range batchJob.TestIDs {
		if job, ok := s.jobs[testID]; ok {
			tests = append(tests, TestStatus{
				TestID:    job.ID,
				Status:    job.Status,
				Progress:  job.Progress,
				Message:   job.Message,
				CreatedAt: job.CreatedAt,
				UpdatedAt: job.UpdatedAt,
			})

			// Count by status
			switch job.Status {
			case "completed":
				completedCount++
			case "failed":
				failedCount++
			case "running":
				runningCount++
			}
		}
	}
	s.mu.RUnlock()

	status := BatchTestStatus{
		BatchID:        batchJob.ID,
		Status:         batchJob.Status,
		Tests:          tests,
		TotalTests:     len(batchJob.TestIDs),
		CompletedTests: completedCount,
		FailedTests:    failedCount,
		RunningTests:   runningCount,
		CreatedAt:      batchJob.CreatedAt,
		UpdatedAt:      batchJob.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Monitor batch status and update when all tests complete
func (s *Server) monitorBatchStatus(batchID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		// Use read lock to check status (doesn't block other operations)
		s.mu.RLock()
		batchJob, exists := s.batchJobs[batchID]
		if !exists {
			s.mu.RUnlock()
			return
		}

		// Check if all tests are complete
		allComplete := true
		anyFailed := false
		failedCount := 0
		completedCount := 0

		for _, testID := range batchJob.TestIDs {
			if job, ok := s.jobs[testID]; ok {
				if job.Status != "completed" && job.Status != "failed" {
					allComplete = false
				}
				if job.Status == "failed" {
					anyFailed = true
					failedCount++
				}
				if job.Status == "completed" {
					completedCount++
				}
			}
		}
		s.mu.RUnlock()

		// Only take write lock if we need to update
		if allComplete {
			s.mu.Lock()
			// Double-check batch still exists
			if batchJob, ok := s.batchJobs[batchID]; ok {
				if anyFailed {
					batchJob.Status = "completed_with_failures"
				} else {
					batchJob.Status = "completed"
				}
				batchJob.UpdatedAt = time.Now()

				// Schedule cleanup after 1 hour
				go func(id string) {
					time.Sleep(1 * time.Hour)
					s.mu.Lock()
					delete(s.batchJobs, id)
					s.mu.Unlock()
					log.Printf("Cleaned up completed batch: %s", id)
				}(batchID)
			}
			s.mu.Unlock()
			return
		}

		// Update timestamp with write lock (brief)
		s.mu.Lock()
		if batchJob, ok := s.batchJobs[batchID]; ok {
			batchJob.UpdatedAt = time.Now()
		}
		s.mu.Unlock()
	}
}

// Execute a test job
func (s *Server) executeTest(job *TestJob) {
	// Acquire semaphore slot (blocks if at max concurrency)
	s.testSemaphore <- struct{}{}
	defer func() {
		<-s.testSemaphore // Release slot when done
		if r := recover(); r != nil {
			s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Panic: %v", r))
		}
	}()

	log.Printf("Starting test %s for URL: %s (concurrent: %d/%d)",
		job.ID, job.Request.URL, len(s.testSemaphore), s.maxConcurrent)

	// Update status to running
	s.updateJob(job.ID, "running", 10, "Initializing browser...")

	// Create browser manager with headless setting from request
	// In production (Docker/deployed), always use headless mode regardless of request
	headless := job.Request.Headless
	if os.Getenv("FORCE_HEADLESS") == "true" {
		headless = true
	}
	bm, err := agent.NewBrowserManager(headless)
	if err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Failed to create browser: %v", err))
		return
	}
	defer bm.Close()

	// Start console logger
	consoleLogger := agent.NewConsoleLogger()
	if err := consoleLogger.StartCapture(bm.GetContext()); err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Failed to start console logger: %v", err))
		return
	}

	s.updateJob(job.ID, "running", 20, "Navigating to URL...")

	// Navigate to URL
	if err := bm.LoadGame(job.Request.URL); err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Navigation failed: %v", err))
		return
	}

	s.updateJob(job.ID, "running", 30, "Capturing initial screenshot...")

	// Capture initial screenshot
	initialScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextInitial)
	if err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Screenshot failed: %v", err))
		return
	}
	if err := initialScreenshot.SaveToTemp(); err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Failed to save screenshot: %v", err))
		return
	}

	s.updateJob(job.ID, "running", 40, "Loading game page...")

	// Wait for page load
	time.Sleep(2 * time.Second)

	// Remove ads and handle cookie consent with improved logic
	log.Printf("Removing ads and handling cookie consent...")
	if err := bm.RemoveAdsAndCookieConsent(); err != nil {
		log.Printf("Warning: Ad blocking/cookie consent failed: %v", err)
	} else {
		log.Printf("Ad blocking and cookie consent handling completed")
		time.Sleep(500 * time.Millisecond)
	}

	detector := agent.NewUIDetector(bm.GetContext())

	s.updateJob(job.ID, "running", 50, "Starting game...")

	// Use vision + DOM to detect and click start button
	log.Printf("Using GPT-4o vision + DOM to detect and click start button...")
	visionDOMDetector, err := agent.NewVisionDOMDetector(bm.GetContext())
	if err != nil {
		log.Printf("Warning: Could not create vision DOM detector: %v", err)
		log.Printf("Skipping start button click...")
	} else {
		// Take screenshot for vision analysis
		visionScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextInitial)
		if err != nil {
			log.Printf("Warning: Could not capture screenshot for vision: %v", err)
		} else {
			// Detect and click start button
			err := visionDOMDetector.DetectAndClickStartButton(visionScreenshot)
			if err != nil {
				log.Printf("Warning: Vision+DOM start button click failed: %v", err)
				log.Printf("Game may require manual start or will auto-start")
			} else {
				log.Printf("✓ Vision+DOM successfully clicked start button")
				time.Sleep(1 * time.Second) // Wait for click to register
			}
		}
	}

	s.updateJob(job.ID, "running", 55, "Waiting for game to load...")

	// Wait for game to load (simple delay for now)
	log.Printf("Waiting 5 seconds for game to load...")
	time.Sleep(5 * time.Second)

	// Detect if game uses canvas or DOM rendering
	log.Printf("Detecting game rendering type...")
	var useCanvasMode bool
	focused, err := detector.FocusGameCanvas()
	if err != nil || !focused {
		log.Printf("No canvas detected or focus failed - using DOM/window event mode")
		useCanvasMode = false
	} else {
		log.Printf("Canvas detected and focused - using canvas event mode")
		useCanvasMode = true
	}

	// Add small delay after detection
	time.Sleep(500 * time.Millisecond)

	// Initialize video recorder
	log.Printf("Initializing video recorder...")
	videoRecorder := agent.NewVideoRecorder(bm.GetContext())

	// Start video recording
	log.Printf("Starting video recording...")
	if err := videoRecorder.StartRecording(); err != nil {
		log.Printf("Warning: Failed to start video recording: %v", err)
		log.Printf("Continuing without video recording...")
	} else {
		log.Printf("✓ Video recording started")
	}

	s.updateJob(job.ID, "running", 60, "Playing game with keyboard controls...")

	// Simulate realistic gameplay with varied interactions over time
	// This gives the AI more meaningful data to evaluate
	gameplayDuration := 10 * time.Second // Much longer gameplay
	gameplayStart := time.Now()

	// Track screenshots captured during gameplay
	var gameplayScreenshots []*agent.Screenshot
	lastScreenshotTime := time.Now()
	screenshotInterval := 2 * time.Second // Capture every 2 seconds

	if useCanvasMode {
		log.Printf("Starting %v of interactive gameplay with canvas keyboard events...", gameplayDuration)
	} else {
		log.Printf("Starting %v of interactive gameplay with window keyboard events...", gameplayDuration)
	}

	// Gameplay loop - send keyboard events to canvas or window
	for time.Since(gameplayStart) < gameplayDuration {
		progress := 60 + int(25*time.Since(gameplayStart).Seconds()/gameplayDuration.Seconds())
		s.updateJob(job.ID, "running", progress, fmt.Sprintf("Playing game... %.0fs elapsed", time.Since(gameplayStart).Seconds()))

		// Capture screenshot every 2 seconds during gameplay
		if time.Since(lastScreenshotTime) >= screenshotInterval {
			screenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextGameplay)
			if err != nil {
				log.Printf("Warning: Failed to capture gameplay screenshot: %v", err)
			} else {
				if err := screenshot.SaveToTemp(); err != nil {
					log.Printf("Warning: Failed to save gameplay screenshot: %v", err)
				} else {
					gameplayScreenshots = append(gameplayScreenshots, screenshot)
					log.Printf("✓ Captured gameplay screenshot (%d total)", len(gameplayScreenshots))
				}
			}
			lastScreenshotTime = time.Now()
		}

		// Send varied key presses (more realistic gameplay)
		gameplayActions := []string{
			"ArrowUp", "ArrowUp", // Jump or move up twice
			"ArrowRight", "ArrowRight", "ArrowRight", // Move right
			"Space", // Action key
			"ArrowLeft", "ArrowLeft", // Move left
			"ArrowDown", // Duck or move down
			"Space", // Action again
			"ArrowRight", // Continue moving
		}

		for _, key := range gameplayActions {
			var sent bool
			var err error

			if useCanvasMode {
				sent, err = detector.SendKeyboardEventToCanvas(key)
			} else {
				sent, err = detector.SendKeyboardEventToWindow(key)
			}

			if err != nil {
				log.Printf("Error sending key %s: %v", key, err)
			} else if !sent {
				log.Printf("Warning: Failed to send key %s", key)
			}
			time.Sleep(150 * time.Millisecond) // Slightly faster inputs
		}

		// Brief pause between action sequences
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("Gameplay simulation completed after %v", time.Since(gameplayStart))

	// Stop video recording
	var videoPath string
	if videoRecorder.IsRecording {
		log.Printf("Stopping video recording...")
		if err := videoRecorder.StopRecording(); err != nil {
			log.Printf("Warning: Failed to stop video recording: %v", err)
		} else {
			log.Printf("✓ Video recording stopped")
			log.Printf("Recorded %d frames over %v", videoRecorder.GetFrameCount(), videoRecorder.GetDuration())

			// Save video to temp file
			log.Printf("Saving video as MP4...")
			videoPath, err = videoRecorder.SaveToTemp()
			if err != nil {
				log.Printf("Warning: Failed to save video: %v", err)
			} else {
				log.Printf("✓ Video saved to: %s", videoPath)
			}
		}
	}

	s.updateJob(job.ID, "running", 70, "Capturing final screenshot...")

	// Wait for game state to settle
	time.Sleep(500 * time.Millisecond)

	// Capture final screenshot
	finalScreenshot, err := agent.CaptureScreenshot(bm.GetContext(), agent.ContextFinal)
	if err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Final screenshot failed: %v", err))
		return
	}
	if err := finalScreenshot.SaveToTemp(); err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Failed to save final screenshot: %v", err))
		return
	}

	s.updateJob(job.ID, "running", 80, "Getting console logs...")

	// Get console logs
	logs := consoleLogger.GetLogs()

	s.updateJob(job.ID, "running", 90, "Evaluating with AI...")

	// Evaluate with LLM
	gameEval, err := evaluator.NewGameEvaluator("")
	if err != nil {
		log.Printf("Warning: Could not initialize evaluator: %v", err)
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Evaluator initialization failed: %v", err))
		return
	}

	// Combine all screenshots: initial, gameplay screenshots, final
	screenshots := []*agent.Screenshot{initialScreenshot}
	screenshots = append(screenshots, gameplayScreenshots...)
	screenshots = append(screenshots, finalScreenshot)
	score, err := gameEval.EvaluateGame(job.ctx, screenshots, logs)
	if err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Evaluation failed: %v", err))
		return
	}

	// Build report
	reportBuilder := reporter.NewReportBuilder(job.Request.URL)
	reportBuilder.AddMetadata("test_id", job.ID)
	reportBuilder.AddMetadata("headless", fmt.Sprintf("%v", job.Request.Headless))
	reportBuilder.SetScreenshots(screenshots)
	reportBuilder.SetConsoleLogs(logs)
	reportBuilder.SetScore(score)

	// Set video URL if video was recorded
	if videoPath != "" {
		// Extract filename from path
		videoFilename := filepath.Base(videoPath)
		videoURL := fmt.Sprintf("/api/videos/%s", videoFilename)
		reportBuilder.SetVideoURL(videoURL)
		log.Printf("Video URL set to: %s", videoURL)
	}

	report, err := reportBuilder.Build()
	if err != nil {
		s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Report build failed: %v", err))
		return
	}

	// Save report to job
	s.mu.Lock()
	if j, ok := s.jobs[job.ID]; ok {
		j.Report = report
		j.Status = "completed"
		j.Progress = 100
		j.Message = "Test completed successfully"
		j.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	// Persist completed test to database
	if err := s.db.CompleteTest(
		job.ID,
		"completed",
		score.OverallScore,
		int(report.SessionDuration),
		report.ReportID,
		report,
	); err != nil {
		log.Printf("Warning: Failed to persist completed test to database: %v", err)
	}

	log.Printf("Test %s completed with score: %d/100", job.ID, score.OverallScore)
}

// Update job status
func (s *Server) updateJob(id, status string, progress int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Status = status
		job.Progress = progress
		job.Message = message
		job.UpdatedAt = time.Now()

		// Persist status updates to database
		if err := s.db.UpdateTestStatus(id, status); err != nil {
			log.Printf("Warning: Failed to update test status in database: %v", err)
		}
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	server := NewServer(port, apiKey)

	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/dreamup.db"
	}
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	server.db = database
	log.Printf("📦 Database initialized: %s", dbPath)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.corsMiddleware(server.handleHealth))
	mux.HandleFunc("/api/config", server.corsMiddleware(server.handleConfig))
	mux.HandleFunc("/api/tests", server.corsMiddleware(server.handleTestSubmit))
	mux.HandleFunc("/api/tests/", server.corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Check if it's a list or single test request
			testID := r.URL.Path[len("/api/tests/"):]
			if testID == "" || testID == "list" {
				server.handleTestList(w, r)
			} else {
				server.handleTestStatus(w, r)
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/reports/", server.corsMiddleware(server.handleTestReport))
	mux.HandleFunc("/api/screenshots/", server.corsMiddleware(server.handleScreenshot))
	mux.HandleFunc("/api/videos/", server.corsMiddleware(server.handleVideo))
	mux.HandleFunc("/api/batch-tests", server.corsMiddleware(server.handleBatchTestSubmit))
	mux.HandleFunc("/api/batch-tests/", server.corsMiddleware(server.handleBatchTestStatus))

	// Serve static files (frontend)
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./frontend/dist"
	}

	// Check if static directory exists
	if _, err := os.Stat(staticDir); err == nil {
		fs := http.FileServer(http.Dir(staticDir))
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Serve index.html for all non-API routes (SPA routing)
			if r.URL.Path != "/" && !fileExists(filepath.Join(staticDir, r.URL.Path)) {
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		}))
		log.Printf("📁 Serving static files from: %s", staticDir)
	} else {
		log.Printf("⚠️  Static directory not found: %s (API-only mode)", staticDir)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("🚀 DreamUp QA API Server v%s", version)
		log.Printf("🌐 Listening on http://localhost:%s", port)
		log.Printf("📊 Health check: http://localhost:%s/health", port)
		log.Printf("📝 API endpoints:")
		log.Printf("   POST   /api/tests            - Submit new test")
		log.Printf("   GET    /api/tests/{id}       - Get test status")
		log.Printf("   GET    /api/tests/list       - List all tests")
		log.Printf("   GET    /api/reports/{id}     - Get test report")
		log.Printf("   POST   /api/batch-tests      - Submit batch test (up to 10 URLs)")
		log.Printf("   GET    /api/batch-tests/{id} - Get batch test status")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}
