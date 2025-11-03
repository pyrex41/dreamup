package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/dreamup/qa-agent/internal/agent"
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
	jobs   map[string]*TestJob
	mu     sync.RWMutex
	port   string
	apiKey string
}

func NewServer(port, apiKey string) *Server {
	return &Server{
		jobs:   make(map[string]*TestJob),
		port:   port,
		apiKey: apiKey,
	}
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

	s.mu.RLock()
	job, exists := s.jobs[testID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Test not found", http.StatusNotFound)
		return
	}

	if job.Report == nil {
		http.Error(w, "Report not ready", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job.Report)
}

// List all tests
func (s *Server) handleTestList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tests := make([]TestStatus, 0, len(s.jobs))
	for _, job := range s.jobs {
		tests = append(tests, TestStatus{
			TestID:    job.ID,
			Status:    job.Status,
			Progress:  job.Progress,
			Message:   job.Message,
			CreatedAt: job.CreatedAt,
			UpdatedAt: job.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tests)
}

// Execute a test job
func (s *Server) executeTest(job *TestJob) {
	defer func() {
		if r := recover(); r != nil {
			s.updateJob(job.ID, "failed", 100, fmt.Sprintf("Panic: %v", r))
		}
	}()

	log.Printf("Starting test %s for URL: %s", job.ID, job.Request.URL)

	// Update status to running
	s.updateJob(job.ID, "running", 10, "Initializing browser...")

	// Create browser manager
	bm, err := agent.NewBrowserManager()
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

	s.updateJob(job.ID, "running", 40, "Handling cookie consent...")

	// Wait for page load
	time.Sleep(2 * time.Second)

	// Handle cookie consent
	detector := agent.NewUIDetector(bm.GetContext())
	if clicked, err := detector.AcceptCookieConsent(); err != nil {
		log.Printf("Cookie consent handling: %v", err)
	} else if clicked {
		log.Printf("Cookie consent accepted")
		time.Sleep(500 * time.Millisecond)
	}

	s.updateJob(job.ID, "running", 50, "Starting game...")

	// Click start button
	if clicked, err := detector.ClickStartButton(); err != nil {
		log.Printf("Start button click: %v", err)
	} else if clicked {
		log.Printf("Game started")
		time.Sleep(500 * time.Millisecond)
	}

	s.updateJob(job.ID, "running", 60, "Simulating gameplay...")

	// Wait for game to render
	time.Sleep(2 * time.Second)

	// Simulate gameplay using chromedp
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
		chromedp.Run(bm.GetContext(), chromedp.KeyEvent(action.key))
		time.Sleep(200 * time.Millisecond)
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

	screenshots := []*agent.Screenshot{initialScreenshot, finalScreenshot}
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
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	server := NewServer(port, apiKey)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.corsMiddleware(server.handleHealth))
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
		log.Printf("   POST   /api/tests        - Submit new test")
		log.Printf("   GET    /api/tests/{id}   - Get test status")
		log.Printf("   GET    /api/tests/list   - List all tests")
		log.Printf("   GET    /api/reports/{id} - Get test report")

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
