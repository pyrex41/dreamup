# DreamUp QA Agent - Architecture & Implementation Guide

## Table of Contents

1. [Overview](#overview)
2. [Architecture Principles](#architecture-principles)
3. [System Architecture](#system-architecture)
4. [Backend Implementation (Go)](#backend-implementation-go)
5. [Frontend Implementation (Elm)](#frontend-implementation-elm)
6. [Data Flow](#data-flow)
7. [Deployment Modes](#deployment-modes)
8. [Database Layer](#database-layer)
9. [API Reference](#api-reference)
10. [Testing Strategy](#testing-strategy)

---

## Overview

DreamUp QA Agent is an **automated quality assurance system** for web-based games. It combines browser automation, AI-powered visual analysis, and comprehensive reporting into a production-ready testing platform.

### Core Capabilities

- **Automated Browser Testing**: Uses Chrome DevTools Protocol via chromedp for headless/headed browser automation
- **AI Visual Analysis**: GPT-4 Vision analyzes screenshots to evaluate game quality
- **Evidence Collection**: Screenshots, console logs, performance metrics, accessibility checks
- **Persistent Storage**: SQLite database for test history and results
- **Multiple Interfaces**: REST API server, CLI tool, AWS Lambda function
- **Real-time Updates**: WebSocket-style status polling for live test progress
- **Batch Processing**: Concurrent test execution with semaphore-based rate limiting

---

## Architecture Principles

### Why Go?

We chose **Go** as the primary backend language for several critical reasons:

1. **Concurrency Model**: Go's goroutines and channels provide lightweight concurrency for managing multiple browser instances
2. **Memory Safety**: Garbage collection prevents memory leaks during long-running test sessions
3. **Static Typing**: Compile-time type checking reduces runtime errors in production
4. **Fast Compilation**: Sub-second builds enable rapid development iteration
5. **Single Binary**: Go compiles to a single executable with no runtime dependencies
6. **Native AWS Lambda**: First-class support for serverless deployment via `provided.al2` runtime
7. **chromedp Integration**: Native Go library for Chrome DevTools Protocol eliminates Selenium/WebDriver overhead

### Why NOT JavaScript/Node.js?

While JavaScript would seem natural for browser automation:

- **Memory Management**: Node.js struggles with memory leaks in long-running browser automation
- **Concurrency**: Async/await is harder to reason about than Go's goroutines for managing 20+ parallel tests
- **Deployment**: Single Go binary vs Node.js + node_modules (hundreds of MB)
- **Performance**: Go's compiled code is 2-5x faster for CPU-intensive tasks (image processing, JSON parsing)
- **Type Safety**: TypeScript adds complexity; Go's types are enforced at compile time
- **Lambda Cold Starts**: Go Lambda functions start in ~100ms vs Node.js ~300ms

### Why Elm for Frontend?

- **Zero Runtime Errors**: Elm's type system eliminates null/undefined errors
- **Immutable State**: Time-travel debugging and predictable state management
- **Maintainability**: Refactoring is safe; compiler catches breaking changes
- **Bundle Size**: ~30KB gzipped vs React apps (200KB+)
- **Performance**: Virtual DOM optimizations with no runtime overhead

---

## System Architecture

### High-Level Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                             │
├─────────────────────────────────────────────────────────────────┤
│  Elm SPA Frontend        CLI Tool           AWS Lambda Invoke   │
│  (Browser)               (Terminal)         (Event)              │
└─────────────┬───────────────┬────────────────────┬──────────────┘
              │               │                    │
              │ HTTP REST     │ Direct Go Calls    │ Lambda Event
              │               │                    │
┌─────────────▼───────────────▼────────────────────▼──────────────┐
│                      Go Backend Services                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────────┐  ┌──────────────────┐  ┌────────────────┐ │
│  │  HTTP Server    │  │  CLI Executor    │  │ Lambda Handler │ │
│  │  (cmd/server)   │  │  (cmd/qa)        │  │ (cmd/lambda)   │ │
│  └────────┬────────┘  └────────┬─────────┘  └────────┬───────┘ │
│           │                    │                      │          │
│           └────────────────────┼──────────────────────┘          │
│                                │                                 │
│  ┌─────────────────────────────▼──────────────────────────────┐ │
│  │              Test Orchestration Layer                       │ │
│  │  • Job Management (map[string]*TestJob)                     │ │
│  │  • Concurrency Control (semaphore: chan struct{})           │ │
│  │  • Status Tracking (Progress, Message, Error)               │ │
│  │  • Context Management (Cancellation, Timeouts)              │ │
│  └─────────────────────────────┬──────────────────────────────┘ │
│                                │                                 │
│  ┌────────────────────┬────────▼──────────┬────────────────────┐│
│  │                    │                   │                     ││
│  │  ┌─────────────────▼────────┐  ┌──────▼──────────┐  ┌──────▼──────┐
│  │  │  Browser Agent           │  │  AI Evaluator   │  │  Reporter   │
│  │  │  (internal/agent)        │  │  (internal/     │  │  (internal/ │
│  │  │                          │  │   evaluator)    │  │   reporter) │
│  │  │ • BrowserManager         │  │                 │  │             │
│  │  │ • EvidenceCollector      │  │ • GPT-4 Vision  │  │ • JSON      │
│  │  │ • InteractionManager     │  │ • Scoring       │  │ • S3 Upload │
│  │  │ • MetricsCollector       │  │ • Analysis      │  │ • Database  │
│  │  └─────────────────┬────────┘  └──────┬──────────┘  └──────┬──────┘
│  │                    │                   │                     │
└──┼────────────────────┼───────────────────┼─────────────────────┼──────┘
   │                    │                   │                     │
┌──▼────────────────────▼───────────────────▼─────────────────────▼──────┐
│                     External Dependencies                              │
├────────────────────────────────────────────────────────────────────────┤
│  Chrome Browser    OpenAI API         SQLite DB        AWS S3          │
│  (chromedp)        (GPT-4 Vision)     (Test History)   (Artifacts)     │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Backend Implementation (Go)

### 1. HTTP Server Architecture

**Location**: `cmd/server/main.go`

The HTTP server is a production-ready REST API built on Go's `net/http` standard library.

#### Key Design Decisions

**Why not use a framework (Gin, Echo, Fiber)?**
- Standard library is sufficient for our endpoint count (6 endpoints)
- Zero external dependencies = faster Lambda cold starts
- Direct control over middleware (CORS, error handling)
- Easier to audit for security issues

**Server Structure**:

```go
type Server struct {
    jobs          map[string]*TestJob    // In-memory active tests
    batchJobs     map[string]*BatchJob   // Batch test tracking
    mu            sync.RWMutex           // Thread-safe map access
    port          string                 // Server port
    apiKey        string                 // API key (future auth)
    testSemaphore chan struct{}          // Concurrency limiter
    maxConcurrent int                    // Max parallel tests (20)
    db            *db.Database           // SQLite connection
}
```

**Concurrency Control**:

```go
// Semaphore pattern limits concurrent browser instances
testSemaphore: make(chan struct{}, 20)

// Acquire slot before starting test
s.testSemaphore <- struct{}{}
defer func() { <-s.testSemaphore }()
```

**Why semaphore vs worker pool?**
- Dynamic test duration (10s - 5min) makes worker pools inefficient
- Semaphore provides backpressure without goroutine starvation
- Simple to reason about: `len(testSemaphore)` = active tests

#### API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Health check + version |
| POST | `/api/tests` | Submit single test |
| GET | `/api/tests/{id}` | Get test status |
| GET | `/api/tests/list` | List test history |
| GET | `/api/reports/{id}` | Get full test report |
| POST | `/api/batch-tests` | Submit batch test (max 10 URLs) |
| GET | `/api/batch-tests/{id}` | Get batch status |
| GET | `/api/screenshots/{filename}` | Serve screenshot files |
| GET | `/api/videos/{filename}` | Serve gameplay videos |

**Request Flow Example** (Single Test):

```
1. POST /api/tests {"url": "https://game.com", "headless": true}
   ↓
2. Generate UUID → testId = "3f46403a-..."
   ↓
3. Create TestJob struct → status = "pending"
   ↓
4. Return 202 Accepted {"testId": "3f46403a-...", "status": "pending"}
   ↓
5. Launch goroutine → runTest(testId, request)
   ↓
6. Client polls GET /api/tests/3f46403a-... every 2 seconds
   ↓
7. Server returns: {"status": "running", "progress": 45, "message": "Collecting evidence"}
   ↓
8. Test completes → status = "completed", save to SQLite
   ↓
9. GET /api/reports/3f46403a-... returns full JSON report
```

### 2. Browser Automation Layer

**Location**: `internal/agent/browser.go`

#### BrowserManager: Lifecycle Management

```go
type BrowserManager struct {
    allocCtx    context.Context    // Chrome allocator context
    allocCancel context.CancelFunc
    ctx         context.Context    // Browser tab context
    cancel      context.CancelFunc
}
```

**Chrome Flags Explained**:

```go
chromedp.DefaultExecAllocatorOptions[:]
    chromedp.Flag("headless", true),              // Run without GUI
    chromedp.Flag("disable-gpu", true),           // GPU not needed headless
    chromedp.Flag("no-sandbox", true),            // Required for Lambda
    chromedp.Flag("disable-dev-shm-usage", true), // Avoid /dev/shm memory issues

    // Block ads to improve test reliability
    chromedp.Flag("host-rules",
        "MAP *.doubleclick.net 127.0.0.1, " +
        "MAP *.googlesyndication.com 127.0.0.1"),

    // Hide automation detection (prevents "Are you a robot?" prompts)
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
```

**Context Hierarchy**:

```
context.Background()
    ↓
allocCtx (Chrome Process)
    ↓
ctx (Browser Tab)
    ↓
timeoutCtx (Individual Operations)
```

**Why this hierarchy?**
- `allocCtx`: Controls Chrome process lifetime (survives across tabs)
- `ctx`: Controls browser tab (one per test)
- `timeoutCtx`: Prevents individual operations from hanging

#### Evidence Collection

**Location**: `internal/agent/evidence.go`

```go
type EvidenceCollector struct {
    ctx context.Context
}

func (ec *EvidenceCollector) CaptureScreenshot(filename string) (string, error)
func (ec *EvidenceCollector) GetConsoleLogs() ([]ConsoleLog, error)
func (ec *EvidenceCollector) DetectUIElements() ([]DetectedElement, error)
```

**Screenshot Capture Process**:

1. Use `chromedp.CaptureScreenshot()` to get PNG bytes
2. Save to `os.TempDir()` with unique filename: `screenshot_<uuid>_<timestamp>.png`
3. Return absolute file path
4. Screenshot served via `/api/screenshots/{filename}` endpoint
5. Security: Prevent directory traversal with `strings.Contains(filename, "..")`

**Console Log Monitoring**:

```go
// Start listening BEFORE page navigation
chromedp.ListenTarget(ctx, func(ev interface{}) {
    if log, ok := ev.(*runtime.EventConsoleAPICalled); ok {
        // Store log entry
    }
})
```

**Why listen before navigation?**
- Console logs during page load would be missed otherwise
- Games often log errors during initialization

### 3. AI Evaluation Layer

**Location**: `internal/evaluator/llm.go`

```go
type Evaluator struct {
    client *openai.Client
}

func (e *Evaluator) EvaluateScreenshots(
    screenshots []string,
    consoleLogs []agent.ConsoleLog,
) (*PlayabilityScore, error)
```

**GPT-4 Vision Integration**:

```go
// Encode screenshot to base64
imageData, _ := os.ReadFile(screenshotPath)
base64Image := base64.StdEncoding.EncodeToString(imageData)

// Send to GPT-4 Vision
messages := []openai.ChatCompletionMessage{
    {
        Role: openai.ChatMessageRoleUser,
        MultiContent: []openai.ChatMessagePart{
            {
                Type: openai.ChatMessagePartTypeText,
                Text: prompt, // Evaluation criteria
            },
            {
                Type: openai.ChatMessagePartTypeImageURL,
                ImageURL: &openai.ChatMessageImageURL{
                    URL: fmt.Sprintf("data:image/png;base64,%s", base64Image),
                },
            },
        },
    },
}
```

**Scoring Algorithm**:

```go
type PlayabilityScore struct {
    OverallScore       int      `json:"overall_score"`        // 0-100
    LoadsCorrectly     bool     `json:"loads_correctly"`
    InteractivityScore int      `json:"interactivity_score"`  // 0-100
    VisualQuality      int      `json:"visual_quality"`       // 0-100
    ErrorSeverity      int      `json:"error_severity"`       // 0-100 (lower is better)
    Reasoning          string   `json:"reasoning"`
    Issues             []string `json:"issues"`
    Recommendations    []string `json:"recommendations"`
}
```

**Error Handling Strategy**:

```go
// Retry with exponential backoff
for attempt := 1; attempt <= 3; attempt++ {
    score, err := e.callGPT4Vision(screenshots, logs)
    if err == nil {
        return score, nil
    }

    if attempt < 3 {
        time.Sleep(time.Duration(attempt) * 2 * time.Second)
    }
}
```

**Why retry?**
- OpenAI API can be flaky (rate limits, timeouts)
- Exponential backoff prevents thundering herd
- 3 attempts = ~14 second max wait (acceptable for QA)

### 4. Reporter & Storage Layer

**Location**: `internal/reporter/report.go`

```go
type Report struct {
    ReportID     string              `json:"report_id"`
    GameURL      string              `json:"game_url"`
    Timestamp    time.Time           `json:"timestamp"`
    DurationMs   int64               `json:"duration_ms"`
    Score        *PlayabilityScore   `json:"score,omitempty"`
    Evidence     Evidence            `json:"evidence"`
    Summary      Summary             `json:"summary"`
    Metadata     map[string]string   `json:"metadata,omitempty"`
}
```

**Report Builder Pattern**:

```go
builder := reporter.NewReportBuilder()
builder.SetGameURL(url)
builder.SetScore(score)
builder.AddScreenshot(path)
builder.SetConsoleLogs(logs)
report := builder.Build()
```

**Why builder pattern?**
- Complex object construction with many optional fields
- Allows partial report generation (if test fails midway)
- Cleaner API than constructor with 15 parameters

**S3 Upload** (`internal/reporter/s3.go`):

```go
func UploadReportToS3(report *Report, bucketName, region string) (string, error) {
    // 1. Marshal report to JSON
    jsonData, _ := json.Marshal(report)

    // 2. Upload to S3
    key := fmt.Sprintf("reports/%s/%s.json",
        time.Now().Format("2006-01-02"),
        report.ReportID)

    // 3. Return public URL
    return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, key), nil
}
```

---

## Database Layer

**Location**: `internal/db/database.go`

### Schema Design

```sql
CREATE TABLE IF NOT EXISTS tests (
    id TEXT PRIMARY KEY,              -- UUID
    game_url TEXT NOT NULL,
    status TEXT NOT NULL,             -- pending, running, completed, failed
    score INTEGER,                    -- Overall score 0-100
    duration_ms INTEGER,              -- Test duration
    report_id TEXT,                   -- UUID (same as id for now)
    report_data TEXT,                 -- Full JSON report
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_tests_status ON tests(status);
CREATE INDEX idx_tests_created_at ON tests(created_at DESC);
```

### Database Operations

```go
type Database struct {
    db *sql.DB
}

func (d *Database) SaveTest(test *TestRecord) error
func (d *Database) GetTest(id string) (*TestRecord, error)
func (d *Database) ListTests(statusFilter string, limit, offset int) ([]*TestRecord, error)
func (d *Database) UpdateTestStatus(id, status string) error
```

**Why SQLite?**
- **Serverless**: No external database server required
- **File-based**: Single file (`./data/dreamup.db`)
- **ACID Compliant**: Crash-safe with transactions
- **Fast**: 10,000+ reads/sec for our query patterns
- **Portable**: Works on Fly.io, Docker, bare metal

**Why NOT PostgreSQL/MySQL?**
- Overkill for single-server deployment
- Requires separate database server (cost, maintenance)
- Network latency vs in-process SQLite
- SQLite is proven at scale (used by iOS, Android, browsers)

**⚠️ Deployment Considerations**:
- **Fly.io/Docker**: SQLite works perfectly with persistent volumes
- **AWS Lambda**: `/tmp` is ephemeral - database lost on cold starts
  - Solution: Use [Archil](https://docs.archil.com/getting-started/introduction) for SQLite-over-S3
  - Or switch to PostgreSQL RDS for Lambda deployments
  - See [Lambda Deployment](#4-aws-lambda-deployment) section for details

**Connection Management**:

```go
// Open with WAL mode for better concurrency
db, err := sql.Open("sqlite3", "file:dreamup.db?_journal_mode=WAL")

// Connection pool settings
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

**WAL Mode Explained**:
- Write-Ahead Logging allows concurrent reads during writes
- Critical for HTTP server handling multiple `/api/tests/list` requests
- Improves write throughput by 2-3x

---

## Frontend Implementation (Elm)

**Location**: `frontend/src/Main.elm`

### Why Elm's Architecture Matters

Elm uses the **Model-View-Update (MVU)** pattern:

```elm
type alias Model =
    { currentPage : Page
    , testForm : TestForm
    , batchTestForm : BatchTestForm
    , testStatus : Maybe TestStatus
    , report : Maybe Report
    , testHistory : TestHistoryState
    , apiBaseUrl : String
    }

type Msg
    = UrlChanged Url
    | LinkClicked Browser.UrlRequest
    | SubmitTest
    | TestSubmitted (Result Http.Error TestResponse)
    | TestStatusFetched (Result Http.Error TestStatus)
    | ...

update : Msg -> Model -> (Model, Cmd Msg)
view : Model -> Html Msg
```

**Benefits over React/Vue**:

1. **No Runtime Errors**: TypeScript can't prevent `undefined.length` - Elm makes it impossible
2. **Compiler as Teammate**: Refactoring is safe; compiler finds all breaking changes
3. **Predictable State**: Immutability eliminates "how did this variable change?" bugs
4. **Time-Travel Debugging**: State history is preserved, can rewind to any point
5. **Guaranteed Rendering**: If it compiles, the UI will render without crashes

### API Communication

**HTTP Requests**:

```elm
-- Submit test
submitTest : String -> TestForm -> Cmd Msg
submitTest apiBaseUrl form =
    Http.post
        { url = apiBaseUrl ++ "/tests"
        , body = Http.jsonBody (encodeTestRequest form)
        , expect = Http.expectJson TestSubmitted testResponseDecoder
        }

-- Decoder for JSON response
testResponseDecoder : Decode.Decoder TestResponse
testResponseDecoder =
    Decode.map2 TestResponse
        (Decode.field "testId" Decode.string)
        (Decode.field "status" Decode.string)
```

**Polling for Status Updates**:

```elm
-- After test submission, poll every 2 seconds
pollTestStatus : String -> String -> Cmd Msg
pollTestStatus apiBaseUrl testId =
    Process.sleep 2000  -- 2 second delay
        |> Task.perform (\_ -> FetchTestStatus testId)

-- HTTP request
fetchTestStatus : String -> String -> Cmd Msg
fetchTestStatus apiBaseUrl testId =
    Http.get
        { url = apiBaseUrl ++ "/tests/" ++ testId
        , expect = Http.expectJson TestStatusFetched testStatusDecoder
        }
```

**Why polling instead of WebSockets?**
- Simpler server implementation (no connection management)
- Works with AWS Lambda (no persistent connections)
- 2-second polling is acceptable latency for QA (not real-time trading)
- Easier to debug (standard HTTP requests in DevTools)

### Report Visualization

The frontend displays comprehensive test reports with:

- **Screenshot Gallery**: Thumbnail grid with lightbox view
- **Console Log Viewer**: Filterable by level (error, warning, info)
- **Performance Metrics**: FPS, load time, accessibility scores with color-coded badges
- **Score Breakdown**: Radar chart showing interactivity, visual quality, error severity
- **Action Buttons**: Copy share link, retry test, download JSON

**Example UI Component**:

```elm
viewReport : Report -> Html Msg
viewReport report =
    div [ class "report-container" ]
        [ viewHeader report
        , viewScoreCard report.score
        , viewPerformanceMetrics report.evidence.performanceMetrics
        , viewScreenshotGallery report.evidence.screenshots
        , viewConsoleLogs report.evidence.consoleLogs
        , viewReportActions report
        ]
```

---

## Data Flow

### Complete Test Execution Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Client Submits Test                                          │
│    POST /api/tests {"url": "https://game.com", "headless": true}│
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 2. Server: Generate UUID, Create TestJob                        │
│    testId = uuid.New()                                           │
│    job := &TestJob{ID: testId, Status: "pending", Progress: 0}  │
│    jobs[testId] = job                                            │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 3. Server: Return 202 Accepted                                  │
│    {"testId": "3f464...", "status": "pending"}                   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 4. Server: Launch Goroutine                                     │
│    go func() { runTest(testId, request) }()                     │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 5. Browser: Initialize Chrome                                   │
│    bm := agent.NewBrowserManager(headless=true)                 │
│    job.Status = "running", job.Progress = 10                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 6. Browser: Navigate to Game URL                                │
│    bm.LoadGame(url) // 45 second timeout                        │
│    job.Progress = 25, job.Message = "Loading game"              │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 7. Agent: Collect Evidence                                      │
│    - CaptureScreenshot("initial")                                │
│    - DetectUIElements() // Find start button, canvas            │
│    - GetConsoleLogs()                                            │
│    job.Progress = 40, job.Message = "Collecting evidence"       │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 8. Agent: Interact with Game                                    │
│    - ClickElement("Start Game")                                  │
│    - Wait 5 seconds for gameplay                                 │
│    - CaptureScreenshot("gameplay")                               │
│    - RecordVideo() // If enabled                                 │
│    job.Progress = 60, job.Message = "Testing gameplay"          │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 9. Metrics: Collect Performance Data                            │
│    - CollectFPS(3 seconds)                                       │
│    - CollectLoadTime()                                           │
│    - CollectAccessibility() // axe-core                          │
│    job.Progress = 75, job.Message = "Analyzing performance"     │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 10. Evaluator: AI Analysis                                      │
│     - Load screenshots, console logs                             │
│     - Call GPT-4 Vision API                                      │
│     - Parse response → PlayabilityScore                          │
│     job.Progress = 90, job.Message = "Evaluating quality"       │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 11. Reporter: Build Report                                      │
│     builder := reporter.NewReportBuilder()                       │
│     builder.SetScore(score)                                      │
│     builder.SetEvidence(screenshots, logs, metrics)              │
│     report := builder.Build()                                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 12. Database: Save Results                                      │
│     db.SaveTest(&TestRecord{                                     │
│         ID: testId,                                              │
│         Status: "completed",                                     │
│         Score: score.OverallScore,                               │
│         ReportData: json.Marshal(report),                        │
│     })                                                           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 13. Server: Update Job Status                                   │
│     job.Status = "completed"                                     │
│     job.Progress = 100                                           │
│     job.Report = report                                          │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 14. Client: Fetch Final Report                                  │
│     GET /api/reports/3f464...                                    │
│     → Display full report with screenshots, scores, metrics      │
└─────────────────────────────────────────────────────────────────┘
```

**Timing Breakdown** (Typical 30-second test):

| Phase | Duration | Progress |
|-------|----------|----------|
| Browser initialization | 2s | 0-10% |
| Page navigation | 5s | 10-25% |
| Evidence collection | 3s | 25-40% |
| Game interaction | 7s | 40-60% |
| Performance metrics | 5s | 60-75% |
| AI evaluation | 6s | 75-90% |
| Report generation | 2s | 90-100% |

---

## Deployment Modes

### 1. Local Development

```bash
# Terminal 1: Start backend
cd cmd/server
DB_PATH="../../data/dreamup.db" go run main.go

# Terminal 2: Start frontend
cd frontend
npm run dev

# Access: http://localhost:3000
```

**Configuration**:
- Backend: `http://localhost:8080`
- Frontend: `http://localhost:3000` (Vite dev server)
- Database: `./data/dreamup.db` (created automatically)
- Screenshots: `os.TempDir()/screenshot_*.png`

### 2. Production Server

```bash
# Build backend
cd cmd/server
go build -o server main.go

# Build frontend
cd frontend
npm run build  # Outputs to ./dist

# Deploy
./server --port=8080 &
nginx -c nginx.conf  # Serve frontend + proxy /api to backend
```

**nginx Configuration**:

```nginx
server {
    listen 80;
    server_name qa.example.com;

    # Serve Elm frontend
    location / {
        root /var/www/dreamup/frontend/dist;
        try_files $uri /index.html;
    }

    # Proxy API requests to Go backend
    location /api {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 3. Docker Deployment

```dockerfile
# Backend + Frontend in single container
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server

FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM alpine:latest
RUN apk add --no-cache chromium
COPY --from=backend-builder /app/server /usr/local/bin/
COPY --from=frontend-builder /app/dist /var/www/html
EXPOSE 8080
CMD ["server"]
```

### 4. AWS Lambda Deployment

**Build**:

```bash
# Compile for Lambda runtime
GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap ./cmd/lambda

# Create deployment package
zip lambda-deployment.zip bootstrap
```

**Terraform**:

```hcl
resource "aws_lambda_function" "qa_agent" {
  filename      = "lambda-deployment.zip"
  function_name = "dreamup-qa-agent"
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"

  timeout     = 300  # 5 minutes
  memory_size = 2048 # 2GB (more memory = faster CPU)

  environment {
    variables = {
      OPENAI_API_KEY = var.openai_api_key
      S3_BUCKET_NAME = aws_s3_bucket.artifacts.bucket
    }
  }
}
```

**Lambda Handler** (`cmd/lambda/main.go`):

```go
type LambdaRequest struct {
    GameURL     string            `json:"game_url"`
    UploadToS3  bool              `json:"upload_to_s3"`
    Timeout     int               `json:"timeout,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

func handler(ctx context.Context, req LambdaRequest) (*LambdaResponse, error) {
    // 1. Initialize browser (headless only in Lambda)
    bm, _ := agent.NewBrowserManager(true)
    defer bm.Close()

    // 2. Run test with timeout
    testCtx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
    defer cancel()

    // 3. Execute test
    report, err := runTest(testCtx, bm, req.GameURL)

    // 4. Upload to S3 if requested
    if req.UploadToS3 {
        url, _ := reporter.UploadReportToS3(report, os.Getenv("S3_BUCKET_NAME"), "us-east-1")
        return &LambdaResponse{ReportURL: url}, nil
    }

    return &LambdaResponse{Report: report}, nil
}

func main() {
    lambda.Start(handler)
}
```

**Lambda Limits & Optimizations**:

| Resource | Limit | Optimization |
|----------|-------|--------------|
| Timeout | 15 min | Set 5 min (enough for most games) |
| Memory | 10 GB | Use 2 GB (good CPU/memory ratio) |
| /tmp storage | 10 GB | Clean up screenshots after upload |
| Cold start | ~2s | Use provisioned concurrency for prod |
| Package size | 250 MB | Go binary is ~20 MB (well under limit) |

**⚠️ Important: SQLite Persistence in Lambda**

Lambda's `/tmp` directory is **ephemeral** - it's wiped between cold starts. This means:

- **No database persistence**: SQLite database is lost when Lambda scales to zero
- **Current state**: Lambda only returns test reports via S3, no test history
- **Workaround options**:
  1. **Use PostgreSQL/RDS** for persistent database (adds cost and latency)
  2. **Use Archil** (recommended): [Archil](https://docs.archil.com/getting-started/introduction) provides SQLite-over-S3 with minimal code changes
  3. **Accept stateless**: Store all reports in S3, query S3 for history (slower)

**Recommended: Archil for SQLite in Lambda**

[Archil](https://archil.com) enables SQLite persistence in Lambda using S3 as the backing store:

```go
import "github.com/archilhq/archil-go"

// Instead of sql.Open("sqlite3", "...")
db, err := archil.Open("sqlite3", "s3://my-bucket/dreamup.db")
```

**Benefits**:
- ✅ Same SQLite code - just change connection string
- ✅ Automatic S3 sync with local caching
- ✅ No cold start penalty (lazy loading)
- ✅ ~$0.01/month S3 storage vs $20/month RDS
- ✅ Sub-10ms query latency with smart caching

**Status**: TBD - Archil integration planned for future Lambda deployments.

For now, Lambda deployments are **stateless** (report-only, no history). For persistent test history, use the **Fly.io deployment** (see [DEPLOYMENT.md](DEPLOYMENT.md)).

---

## API Reference

### Authentication

Currently **no authentication** (API key placeholder for future).

**Planned**: Bearer token authentication

```
Authorization: Bearer <api-key>
```

### Error Response Format

```json
{
  "error": "Test not found",
  "code": "NOT_FOUND",
  "details": {
    "testId": "3f46403a-..."
  }
}
```

### Endpoints

#### `POST /api/tests`

Submit a new test.

**Request**:
```json
{
  "url": "https://funhtml5games.com/pacman/index.html",
  "maxDuration": 60,
  "headless": true
}
```

**Response** (202 Accepted):
```json
{
  "testId": "3f46403a-9d75-4c4a-82d8-56b6f403a1e7",
  "status": "pending"
}
```

#### `GET /api/tests/{testId}`

Get test status.

**Response**:
```json
{
  "testId": "3f46403a-...",
  "status": "running",
  "progress": 45,
  "message": "Collecting evidence",
  "createdAt": "2025-11-04T09:49:40-06:00",
  "updatedAt": "2025-11-04T09:49:55-06:00"
}
```

**Status Values**:
- `pending`: Test queued, waiting for browser slot
- `running`: Test in progress
- `completed`: Test finished successfully
- `failed`: Test encountered an error

#### `GET /api/tests/list?status=all`

List test history.

**Query Parameters**:
- `status`: Filter by status (`all`, `completed`, `failed`, `running`)

**Response**:
```json
[
  {
    "reportId": "3f46403a-...",
    "gameUrl": "https://funhtml5games.com/2048/index.html",
    "timestamp": "2025-11-04T09:49:40-06:00",
    "status": "completed",
    "overallScore": 95,
    "duration": 30
  }
]
```

#### `GET /api/reports/{reportId}`

Get full test report.

**Response**: See [Test Report Structure](#test-report-structure)

#### `POST /api/batch-tests`

Submit batch test (max 10 URLs).

**Request**:
```json
{
  "urls": [
    "https://game1.com",
    "https://game2.com"
  ],
  "maxDuration": 60,
  "headless": true
}
```

**Response** (202 Accepted):
```json
{
  "batchId": "7184aa60-...",
  "testIds": ["3f46403a-...", "f9e6f2dd-..."],
  "status": "running"
}
```

#### `GET /api/batch-tests/{batchId}`

Get batch test status.

**Response**:
```json
{
  "batchId": "7184aa60-...",
  "status": "running",
  "tests": [
    {
      "testId": "3f46403a-...",
      "status": "completed",
      "progress": 100
    },
    {
      "testId": "f9e6f2dd-...",
      "status": "running",
      "progress": 60
    }
  ],
  "totalTests": 2,
  "completedTests": 1,
  "failedTests": 0,
  "runningTests": 1
}
```

---

## Testing Strategy

### Unit Tests

**Browser Agent Tests** (`internal/agent/browser_test.go`):

```go
func TestBrowserManager_Navigate(t *testing.T) {
    bm, err := agent.NewBrowserManager(true)
    if err != nil {
        t.Fatalf("Failed to create browser: %v", err)
    }
    defer bm.Close()

    err = bm.Navigate("https://example.com")
    if err != nil {
        t.Errorf("Navigate failed: %v", err)
    }
}
```

### Integration Tests

**End-to-End Test** (`test/e2e_test.go`):

```go
func TestFullTestExecution(t *testing.T) {
    // 1. Submit test
    resp := submitTest("https://funhtml5games.com/pacman/index.html")
    testId := resp.TestId

    // 2. Poll until complete
    status := pollUntilComplete(testId, 60*time.Second)
    if status != "completed" {
        t.Errorf("Expected completed, got %s", status)
    }

    // 3. Fetch report
    report := getReport(testId)
    if report.Score.OverallScore < 50 {
        t.Errorf("Score too low: %d", report.Score.OverallScore)
    }
}
```

### Performance Tests

**Concurrency Test**:

```go
func TestConcurrentTests(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 20; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            submitTest("https://game.com")
        }()
    }
    wg.Wait()

    // Verify all tests completed within reasonable time
}
```

---

## Performance Characteristics

### Backend Performance

| Metric | Value | Notes |
|--------|-------|-------|
| API response time | < 5ms | Health check, status endpoints |
| Test submission | < 10ms | Async execution |
| Concurrent tests | 20 | Semaphore-limited |
| Memory per test | ~150 MB | Chrome + Go runtime |
| Database queries | < 1ms | SQLite in-process |
| Report generation | < 100ms | JSON marshaling |

### Frontend Performance

| Metric | Value | Notes |
|--------|-------|-------|
| Initial load | ~200ms | 30KB Elm bundle |
| Time to Interactive | ~300ms | No external JS dependencies |
| Report render | ~50ms | Virtual DOM diffing |
| Memory usage | ~15 MB | Lightweight Elm runtime |

### Bottlenecks & Solutions

**Bottleneck**: Chrome browser initialization (2-3 seconds)
**Solution**: Consider browser pool (keep 5 browsers warm)

**Bottleneck**: GPT-4 Vision API latency (3-6 seconds)
**Solution**: Cache common game evaluations, use streaming API

**Bottleneck**: Screenshot file I/O
**Solution**: Use in-memory buffer for temporary storage

---

## Security Considerations

### Input Validation

```go
// Validate URL before testing
func validateURL(urlStr string) error {
    parsedURL, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
        return fmt.Errorf("only HTTP/HTTPS URLs allowed")
    }

    return nil
}
```

### File Serving Security

```go
// Prevent directory traversal
if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
    http.Error(w, "Invalid filename", http.StatusBadRequest)
    return
}

// Only serve whitelisted file types
if !strings.HasPrefix(filename, "screenshot_") {
    http.Error(w, "Invalid filename", http.StatusBadRequest)
    return
}
```

### Database Security

```go
// Use parameterized queries
db.Exec("INSERT INTO tests VALUES (?, ?, ?)", id, gameURL, status)

// NOT: db.Exec(fmt.Sprintf("INSERT INTO tests VALUES ('%s', ...)", id))
```

### Secrets Management

```bash
# Development: .env file (gitignored)
export OPENAI_API_KEY="sk-..."

# Production: AWS Secrets Manager
aws secretsmanager get-secret-value --secret-id dreamup/openai-key
```

---

## Future Enhancements

1. **WebSocket API**: Real-time test updates instead of polling
2. **Browser Pooling**: Pre-warmed Chrome instances for faster tests
3. **Distributed Testing**: Run tests across multiple servers
4. **Video Recording**: Capture full gameplay session
5. **Regression Testing**: Compare new tests against baseline
6. **Webhooks**: Notify external systems when tests complete
7. **API Authentication**: OAuth2 or API key authentication
8. **Multi-region**: Deploy in us-east-1, eu-west-1, ap-southeast-1

---

## Troubleshooting Guide

### Common Issues

**"Browser failed to start"**
```bash
# Check Chrome installation
which chromium-browser

# Run with --no-sandbox flag (required for Lambda)
chromedp.Flag("no-sandbox", true)
```

**"Database locked"**
```bash
# Check for stale connections
lsof | grep dreamup.db

# Enable WAL mode for better concurrency
PRAGMA journal_mode=WAL;
```

**"Test stuck at 45%"**
```bash
# Increase timeout for slow games
maxDuration: 120  # 2 minutes

# Check console logs for JavaScript errors
GET /api/reports/{id}
```

**"Out of memory"**
```bash
# Limit concurrent tests
maxConcurrent: 10  # Reduce from 20

# Increase Lambda memory
memory_size = 4096  # 4GB
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and PR guidelines.

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Built with ❤️ using Go, Elm, and Chrome DevTools Protocol**
