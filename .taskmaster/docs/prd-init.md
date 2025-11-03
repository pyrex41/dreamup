# DreamUp: Browser Game QA Pipeline - Product Requirements Document

**Version:** 2.0  
**Date:** November 3, 2025  
**Project Owner:** Matt Smith (matt.smith@superbuilders.school)  
**Timeline:** 3-5 days core implementation  
**Status:** Planning Phase

---

## Executive Summary

Build an autonomous AI agent that tests browser-based games by simulating user interactions, capturing evidence, and generating structured quality assessments. This system will validate game playability and provide actionable feedback for the DreamUp game-building pipeline.

**Key Metrics:**
- Test 3+ diverse browser games end-to-end
- 80%+ accuracy on playability assessment
- <5 minutes execution time per game
- Deployable to AWS Lambda environment

---

## 1. Problem Statement

### Current State
DreamUp generates browser games through an AI pipeline but lacks automated quality assurance. Manual testing is time-consuming and doesn't scale with automated game generation.

### Desired State
An autonomous agent that can:
- Load any web-hosted game
- Interact with game controls intelligently
- Capture visual and log evidence
- Generate structured quality reports
- Enable feedback loops for game-building improvement

### Business Impact
- **Automation:** Eliminate manual QA bottleneck
- **Scale:** Test games as fast as they're generated
- **Quality:** Consistent evaluation criteria across all games
- **Iteration:** Enable rapid game-building agent improvement

---

## 2. Technical Architecture

### 2.1 Language & Runtime Decision

**Recommended: Go**
- ✅ Excellent Lambda support (fast cold starts, native compilation)
- ✅ Strong browser automation via chromedp (official Chrome DevTools Protocol)
- ✅ Robust error handling and timeouts
- ✅ Single binary deployment
- ✅ Strong typing and tooling
- ⚠️ Slightly more verbose than dynamic languages

**Alternative: TypeScript/Node.js**
- ✅ Rich browser automation ecosystem (Playwright, Puppeteer, Stagehand)
- ✅ Browserbase has first-class support
- ✅ Vercel AI SDK integration
- ⚠️ Lambda cold starts slower than Go
- ⚠️ Your stated preference against JavaScript

**Not Recommended: Elm**
- ❌ No mature browser automation libraries
- ❌ No Chrome DevTools Protocol bindings
- ❌ elm-pages scripts better suited for build-time tasks
- ❌ Would require FFI to Node/Go for core functionality

**Decision Rationale:** Given Lambda deployment requirements and browser automation needs, **Go with chromedp** offers the best balance of performance, maintainability, and your language preferences.

### 2.2 Core Stack

```
┌─────────────────────────────────────────┐
│         Lambda Handler (Go)             │
├─────────────────────────────────────────┤
│  ┌────────────┐      ┌───────────────┐  │
│  │   Agent    │────▶ │   Chromedp    │  │
│  │ Controller │      │  (Headless    │  │
│  └────────────┘      │   Chrome)     │  │
│        │             └───────────────┘  │
│        ▼                      │         │
│  ┌────────────┐              │         │
│  │    LLM     │              │         │
│  │ Evaluator  │◀─────────────┘         │
│  │ (Claude)   │     Screenshots        │
│  └────────────┘        & Logs          │
│        │                               │
│        ▼                               │
│  ┌────────────┐                        │
│  │   Report   │                        │
│  │ Generator  │                        │
│  └────────────┘                        │
└─────────────────────────────────────────┘
         │
         ▼
    S3 Bucket (artifacts)
```

**Technology Choices:**
- **Runtime:** Go 1.21+
- **Browser:** chromedp (Chrome DevTools Protocol)
- **LLM:** Claude 3.5 Sonnet via Anthropic API
- **Storage:** S3 for screenshots/reports
- **Packaging:** Single Go binary or containerized Lambda

### 2.3 Directory Structure

```
dreamup-qa/
├── cmd/
│   └── qa/
│       └── main.go              # Lambda handler entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go            # Core agent orchestration
│   │   ├── browser.go          # Chrome automation
│   │   ├── interactions.go     # Game interaction logic
│   │   └── evidence.go         # Screenshot/log capture
│   ├── evaluator/
│   │   ├── llm.go             # Claude API integration
│   │   ├── prompts.go         # Structured evaluation prompts
│   │   └── scoring.go         # Confidence calculation
│   ├── reporter/
│   │   ├── json.go            # JSON report generation
│   │   └── storage.go         # S3 artifact upload
│   └── types/
│       └── models.go          # Shared data structures
├── pkg/
│   └── config/
│       └── config.go          # Configuration management
├── test/
│   ├── games/                 # Sample game URLs
│   └── fixtures/              # Mock responses
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── Dockerfile                 # Optional containerization
```

---

## 3. Functional Requirements

### 3.1 Browser Automation Agent

**FR-1.1: Game Loading**
- **Input:** Game URL (string)
- **Output:** Browser instance with loaded page
- **Requirements:**
  - Navigate to URL within 30 seconds
  - Wait for DOMContentLoaded event
  - Detect successful render (non-blank viewport)
  - Handle redirects automatically
  - Timeout after 45 seconds with structured error

**FR-1.2: UI Pattern Detection**
- **Patterns to Detect:**
  - Start/Play buttons (common text: "Start", "Play", "Begin", "New Game")
  - Menu screens (navigation elements)
  - Game canvas (HTML5 canvas, WebGL, or game container divs)
  - Game over screens (text patterns: "Game Over", "Try Again", "Restart")
- **Method:** 
  - CSS selector matching
  - OCR on screenshots for text-based detection
  - Z-index analysis for modal overlays

**FR-1.3: Interaction Sequence**
```go
type InteractionPlan struct {
    Actions []Action
    MaxDuration time.Duration
    SuccessCriteria []Criterion
}

type Action struct {
    Type        string        // "click", "keypress", "wait"
    Target      string        // selector or key
    WaitAfter   time.Duration
    Screenshot  bool
}
```

**Standard Sequence:**
1. Initial screenshot (baseline)
2. Click start button (if detected)
3. Wait 2s for game to initialize
4. Screenshot (game start state)
5. Simulate gameplay:
   - Arrow keys (up/down/left/right) - 3-5 presses
   - Spacebar - 2-3 presses
   - Mouse clicks at strategic positions
   - Each action: 500ms-1s pause
6. Screenshot every 10 seconds
7. Final screenshot after 60s or game over
8. Maximum 5 screenshots per session

**FR-1.4: Error Detection**
- **Console Monitoring:**
  - JavaScript errors
  - Failed network requests
  - Console.error() calls
- **Visual Detection:**
  - White screen (blank page)
  - Error messages in viewport
  - Frozen frame (compare screenshots)
- **Timeout Handling:**
  - Max execution: 5 minutes per game
  - Per-action timeout: 30 seconds
  - Screenshot capture timeout: 10 seconds

### 3.2 Evidence Capture

**FR-2.1: Screenshot Management**
```go
type Screenshot struct {
    Timestamp   time.Time
    FilePath    string
    S3URL       string
    Context     string  // "initial", "gameplay", "error", "final"
    Width       int
    Height      int
}
```

- **Format:** PNG (lossless for LLM analysis)
- **Resolution:** 1280x720 (720p)
- **Naming:** `{gameID}_{timestamp}_{context}.png`
- **Storage:** Local temp during test, upload to S3 afterward
- **Compression:** Use PNG compression level 6

**FR-2.2: Log Aggregation**
```go
type LogEntry struct {
    Timestamp   time.Time
    Level       string  // "info", "warn", "error"
    Source      string  // "console", "network", "agent"
    Message     string
    StackTrace  string  // if applicable
}
```

- Capture all console.log/warn/error
- Network request failures (4xx/5xx)
- Agent decision points (actions taken)
- Performance metrics (optional)

**FR-2.3: Artifact Structure**
```
s3://dreamup-qa-artifacts/
├── {game-id}/
│   ├── {timestamp}/
│   │   ├── screenshots/
│   │   │   ├── 001_initial.png
│   │   │   ├── 002_gameplay.png
│   │   │   └── 003_final.png
│   │   ├── logs.json
│   │   └── report.json
```

### 3.3 AI Evaluation

**FR-3.1: Evaluation Prompt Structure**
```
You are evaluating a browser game for playability. Analyze the provided evidence:

SCREENSHOTS: {screenshot_count} images from game session
CONSOLE LOGS: {log_summary}
INTERACTION HISTORY: {actions_taken}

Assess the following:

1. LOAD SUCCESS (0-100)
   - Did the game render properly?
   - Is the game canvas/content visible?
   - Are there any error messages?

2. CONTROL RESPONSIVENESS (0-100)
   - Did interactions cause visible changes?
   - Are animations/physics working?
   - Evidence of broken controls?

3. STABILITY (0-100)
   - Any crashes or freezes?
   - Console errors during gameplay?
   - Did the session complete normally?

Respond in JSON format:
{
  "load_success": {"score": 0-100, "reasoning": "..."},
  "control_responsiveness": {"score": 0-100, "reasoning": "..."},
  "stability": {"score": 0-100, "reasoning": "..."},
  "overall_playability": 0-100,
  "confidence": 0-100,
  "issues": ["issue1", "issue2"],
  "recommendations": ["rec1", "rec2"]
}
```

**FR-3.2: Vision Analysis Requirements**
- Send screenshots as base64-encoded images
- Maximum 5 images per evaluation (to control token cost)
- Prioritize: initial + error frames + final frame
- Include timestamp context with each image

**FR-3.3: Scoring Algorithm**
```go
type PlayabilityScore struct {
    LoadSuccess          int      // 0-100
    ControlResponsive    int      // 0-100
    Stability            int      // 0-100
    OverallPlayability   int      // weighted average
    Confidence           int      // 0-100 (LLM's certainty)
    PassThreshold        int      // 70 (configurable)
}

func (s *PlayabilityScore) OverallScore() int {
    return (s.LoadSuccess*0.3 + 
            s.ControlResponsive*0.4 + 
            s.Stability*0.3)
}

func (s *PlayabilityScore) Pass() bool {
    return s.OverallScore() >= s.PassThreshold && 
           s.Confidence >= 60
}
```

### 3.4 Report Generation

**FR-4.1: JSON Output Format**
```json
{
  "test_id": "uuid-v4",
  "game_url": "https://example.com/game",
  "timestamp": "2025-11-03T14:30:00Z",
  "status": "completed|failed|timeout",
  "execution_time_ms": 45000,
  "playability_score": 85,
  "confidence": 78,
  "pass": true,
  "metrics": {
    "load_success": 95,
    "control_responsiveness": 80,
    "stability": 90
  },
  "issues": [
    {
      "severity": "warning|error|critical",
      "description": "Minor rendering delay on initial load",
      "evidence_screenshot": "s3://bucket/001.png"
    }
  ],
  "recommendations": [
    "Consider optimizing asset loading",
    "Add loading indicator"
  ],
  "evidence": {
    "screenshots": [
      {
        "url": "s3://bucket/001.png",
        "timestamp": "2025-11-03T14:30:15Z",
        "context": "initial"
      }
    ],
    "log_summary": {
      "total_entries": 42,
      "errors": 2,
      "warnings": 5
    },
    "interaction_log": [
      {"action": "click", "target": "#start-btn", "success": true},
      {"action": "keypress", "target": "ArrowUp", "success": true}
    ]
  },
  "artifacts": {
    "full_report_url": "s3://bucket/report.json",
    "screenshot_dir": "s3://bucket/screenshots/"
  }
}
```

**FR-4.2: Human-Readable Summary**
- Generate markdown summary for quick review
- Include embedded screenshot links
- Highlight critical issues
- Provide quick verdict (✅ Pass / ❌ Fail / ⚠️ Warning)

---

## 4. Non-Functional Requirements

### 4.1 Performance
- **NFR-1:** Single game test completes in <5 minutes (95th percentile)
- **NFR-2:** Cold start Lambda execution <10 seconds
- **NFR-3:** Screenshot capture <3 seconds per image
- **NFR-4:** LLM evaluation <15 seconds (with caching)

### 4.2 Reliability
- **NFR-5:** 90% success rate on valid game URLs
- **NFR-6:** Graceful degradation: missing screenshots don't fail entire test
- **NFR-7:** Automatic retry on transient failures (network, browser crash)
- **NFR-8:** All errors logged with structured context

### 4.3 Cost Efficiency
- **NFR-9:** <$0.10 per game test (target <$0.05)
  - Lambda compute: ~$0.01
  - Claude API: ~$0.02-0.04 (5 images + text)
  - S3 storage: <$0.001
- **NFR-10:** LLM response caching for identical screenshots
- **NFR-11:** Use Claude 3 Haiku for log analysis, Sonnet for vision

### 4.4 Maintainability
- **NFR-12:** Code coverage >70% for core agent logic
- **NFR-13:** All public functions have godoc comments
- **NFR-14:** Configuration via environment variables
- **NFR-15:** Logging uses structured format (JSON)

---

## 5. Interface Specifications

### 5.1 Lambda Handler Interface

**Input Event:**
```json
{
  "game_url": "https://example.com/game",
  "config": {
    "max_duration_seconds": 300,
    "screenshot_count": 5,
    "interaction_strategy": "standard|minimal|aggressive"
  }
}
```

**Response:**
```json
{
  "statusCode": 200,
  "body": "{...report JSON...}"
}
```

**Error Response:**
```json
{
  "statusCode": 500,
  "body": {
    "error": "browser_crash",
    "message": "Chrome process exited unexpectedly",
    "context": {...}
  }
}
```

### 5.2 CLI Interface (Local Testing)

```bash
# Basic usage
./qa-agent test --url https://example.com/game

# With options
./qa-agent test \
  --url https://example.com/game \
  --output ./results/game-test.json \
  --screenshots ./results/screenshots/ \
  --max-duration 300 \
  --headless=false  # for debugging

# Batch mode
./qa-agent batch --input games.txt --output-dir ./results/

# Validate report
./qa-agent validate --report results/game-test.json
```

### 5.3 Configuration

**Environment Variables:**
```bash
ANTHROPIC_API_KEY=sk-ant-...
AWS_S3_BUCKET=dreamup-qa-artifacts
AWS_REGION=us-east-1
CHROME_PATH=/usr/bin/chromium  # optional
MAX_TEST_DURATION=300
PASS_THRESHOLD=70
LOG_LEVEL=info
```

**Config File (config.yaml):**
```yaml
browser:
  headless: true
  viewport:
    width: 1280
    height: 720
  timeout: 45s

interaction:
  strategy: standard
  actions_per_test: 10
  screenshot_interval: 10s

evaluation:
  model: claude-3-5-sonnet-20241022
  max_tokens: 4000
  temperature: 0.1

storage:
  s3_bucket: dreamup-qa-artifacts
  retention_days: 30
```

---

## 6. Interaction Strategies

### 6.1 Strategy: Standard (Default)
```go
type StandardStrategy struct {
    Actions []Action{
        {Type: "wait", Duration: 2},
        {Type: "screenshot", Context: "initial"},
        {Type: "click", Selector: "button:contains('Start')"},
        {Type: "wait", Duration: 3},
        {Type: "screenshot", Context: "post-start"},
        {Type: "keypress", Key: "ArrowRight", Repeat: 3},
        {Type: "wait", Duration: 2},
        {Type: "keypress", Key: "Space", Repeat: 2},
        {Type: "screenshot", Context: "mid-gameplay"},
        {Type: "keypress", Key: "ArrowUp", Repeat: 2},
        {Type: "wait", Duration: 5},
        {Type: "screenshot", Context: "final"},
    }
}
```

### 6.2 Strategy: Minimal (Fast Tests)
- 2 screenshots only (initial + final)
- 5 total interactions
- 60 second max duration

### 6.3 Strategy: Aggressive (Stress Testing)
- Rapid key presses (100ms intervals)
- Mouse movements and clicks
- Multiple restarts
- 10 screenshots

### 6.4 Adaptive Strategy (Future)
- LLM suggests next action based on current state
- Builds interaction tree during test
- Requires vision model for real-time analysis

---

## 7. Error Handling & Resilience

### 7.1 Error Classification

```go
type ErrorCategory string

const (
    ErrBrowserCrash     ErrorCategory = "browser_crash"
    ErrGameNotLoaded    ErrorCategory = "game_not_loaded"
    ErrNetworkFailure   ErrorCategory = "network_failure"
    ErrTimeout          ErrorCategory = "timeout"
    ErrLLMFailure       ErrorCategory = "llm_failure"
    ErrStorageFailure   ErrorCategory = "storage_failure"
)

type TestError struct {
    Category    ErrorCategory
    Message     string
    Recoverable bool
    Context     map[string]interface{}
}
```

### 7.2 Retry Logic

```go
type RetryPolicy struct {
    MaxAttempts     int           // 3
    InitialDelay    time.Duration // 2s
    MaxDelay        time.Duration // 30s
    Multiplier      float64       // 2.0 (exponential backoff)
    RetryableErrors []ErrorCategory
}
```

**Retryable Errors:**
- Network timeouts
- Browser initialization failures
- Transient LLM API errors (rate limits, 5xx)

**Non-Retryable Errors:**
- Invalid URL format
- S3 permission denied
- Malformed configuration

### 7.3 Graceful Degradation

**Scenario: Screenshot Capture Fails**
- Log warning
- Continue test with remaining screenshots
- Lower confidence score in report
- Still attempt LLM evaluation

**Scenario: LLM API Unavailable**
- Fall back to heuristic scoring:
  - Load success: Check for error messages in console
  - Responsiveness: Detect state changes between screenshots
  - Stability: Count console errors
- Report with confidence = 30 (low)

**Scenario: Browser Crashes Mid-Test**
- Save all artifacts collected so far
- Generate partial report
- Mark status as "incomplete"
- Return actionable error message

---

## 8. Testing Strategy

### 8.1 Unit Tests
- **Browser utilities:** Mock Chrome DevTools Protocol
- **Interaction logic:** Test action sequencing
- **Report generation:** Validate JSON structure
- **Error handling:** Simulate failure modes

### 8.2 Integration Tests
- **Full pipeline:** Real browser + mock LLM
- **LLM integration:** Real API with test fixtures
- **S3 storage:** LocalStack or test bucket

### 8.3 End-to-End Test Games

| Game Type | URL | Expected Result | Key Validations |
|-----------|-----|-----------------|-----------------|
| Simple Puzzle | tic-tac-toe.html | Pass (90+) | Click interactions work |
| Platformer | mario-clone.html | Pass (85+) | Keyboard controls responsive |
| Idle Game | cookie-clicker.html | Pass (80+) | Minimal interaction required |
| Broken Game | error-game.html | Fail (<50) | Detects console errors |
| Complex RPG | rpg-demo.html | Pass (75+) | Multi-screen navigation |
| Blank Page | blank.html | Fail (<30) | Detects non-functional game |

**Test Game Sources:**
- itch.io HTML5 games (permissive licensing)
- Custom test fixtures in `/test/games/`
- Intentionally broken versions of working games

### 8.4 Performance Benchmarks
```bash
# Run benchmark suite
go test -bench=. -benchmem ./internal/agent

# Expected results:
BenchmarkFullTest/simple-game     2  650ms/op   120MB/alloc
BenchmarkFullTest/complex-game    1  2800ms/op  280MB/alloc
BenchmarkScreenshot              50   45ms/op    5MB/alloc
BenchmarkLLMEvaluation           10  1200ms/op   2MB/alloc
```

---

## 9. Implementation Plan

### Day 1: Foundation & Browser Setup
**Goal:** Browser launches, loads URL, takes screenshots

**Tasks:**
- [ ] Initialize Go module with chromedp
- [ ] Implement browser manager (launch, navigate, shutdown)
- [ ] Screenshot capture function
- [ ] Basic CLI interface
- [ ] Test with static HTML page

**Deliverable:** Agent loads a URL and saves 1 screenshot

### Day 2: Interaction System
**Goal:** Agent executes action sequences

**Tasks:**
- [ ] Implement Action types (click, keypress, wait)
- [ ] Build InteractionPlan executor
- [ ] Add UI pattern detection (start buttons)
- [ ] Console log capture
- [ ] Test with simple game (tic-tac-toe)

**Deliverable:** Agent plays through simple game with 3+ interactions

### Day 3: LLM Evaluation
**Goal:** AI assessment integrated, JSON output

**Tasks:**
- [ ] Anthropic API client
- [ ] Evaluation prompt templates
- [ ] Screenshot → base64 encoding
- [ ] JSON parsing and validation
- [ ] Scoring calculation
- [ ] Test with 2 different games

**Deliverable:** Complete report JSON with AI assessment

### Day 4: Error Handling & Polish
**Goal:** Robust failure modes, tested on 3+ games

**Tasks:**
- [ ] Timeout handling
- [ ] Retry logic
- [ ] Graceful degradation
- [ ] Error categorization
- [ ] Test suite with broken games
- [ ] Logging improvements

**Deliverable:** Handles all failure modes gracefully

### Day 5: Lambda & Documentation
**Goal:** Lambda-ready deployment, complete docs

**Tasks:**
- [ ] Lambda handler wrapper
- [ ] S3 artifact upload
- [ ] Environment config loading
- [ ] README with setup guide
- [ ] Architecture documentation
- [ ] Demo video (5 min)

**Deliverable:** Lambda-deployable package + docs

### Days 6-7: Stretch Features (Optional)
- [ ] GIF recording via FFmpeg
- [ ] Batch testing mode
- [ ] Web dashboard (simple HTML)
- [ ] Advanced metrics (FPS, load time)
- [ ] Parallel test execution

---

## 10. Dependencies & Setup

### 10.1 Go Dependencies
```go.mod
module github.com/dreamup/qa-agent

go 1.21

require (
    github.com/chromedp/chromedp v0.9.3
    github.com/anthropics/anthropic-sdk-go v0.1.0
    github.com/aws/aws-sdk-go-v2 v1.24.0
    github.com/aws/aws-sdk-go-v2/service/s3 v1.48.0
    github.com/google/uuid v1.5.0
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    go.uber.org/zap v1.26.0
)
```

### 10.2 Local Development Setup
```bash
# Install Go 1.21+
brew install go

# Install Chrome (for chromedp)
brew install --cask google-chrome

# Clone repo
git clone https://github.com/dreamup/qa-agent
cd qa-agent

# Install dependencies
go mod download

# Set environment variables
export ANTHROPIC_API_KEY=your-key
export AWS_PROFILE=dreamup-dev

# Run tests
go test ./...

# Build binary
go build -o qa-agent cmd/qa/main.go

# Test locally
./qa-agent test --url https://example.com/game --headless=false
```

### 10.3 Lambda Deployment
```bash
# Build for Lambda (Linux)
GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/qa/main.go
zip lambda.zip bootstrap

# Deploy via AWS CLI
aws lambda update-function-code \
  --function-name dreamup-qa-agent \
  --zip-file fileb://lambda.zip

# Or use Dockerfile
docker build -t dreamup-qa-agent .
# Push to ECR and deploy
```

---

## 11. Cost Analysis

### Per-Test Cost Breakdown
```
Component               Cost      Notes
─────────────────────────────────────────────────
Lambda (256MB, 60s)    $0.001    Free tier: 1M requests/mo
Claude API:
  - 5 images           $0.024    5 × 1280×720 @ $0.048/image
  - Text input (2K)    $0.006    Prompt + logs
  - Text output (1K)   $0.015    JSON report
S3 Storage             $0.0001   5 images @ 200KB each
S3 Requests            $0.00001  6 PUTs
─────────────────────────────────────────────────
Total per test:        ~$0.046
─────────────────────────────────────────────────

Monthly (1000 tests):  $46
Monthly (10K tests):   $460
```

### Cost Optimization Strategies
1. **Response Caching:** Store LLM responses for identical screenshots
2. **Image Compression:** Reduce PNG size without quality loss (ImageMagick)
3. **Selective Vision:** Only send screenshots that show meaningful changes
4. **Model Selection:** Use Claude 3 Haiku ($0.25/MTok) for text-only analysis
5. **Batch API:** Use batch API for non-urgent tests (50% discount)

---

## 12. Security & Compliance

### 12.1 Security Considerations
- **Sandboxed Browser:** Chrome runs in container with no network access to internal resources
- **URL Validation:** Reject non-HTTP(S) URLs, prevent SSRF
- **Resource Limits:** Timeout prevents infinite loops, memory limits prevent DOS
- **API Key Management:** Store in AWS Secrets Manager, rotate regularly
- **Output Sanitization:** Strip sensitive data from logs/screenshots

### 12.2 Privacy
- **No User Data:** System tests public URLs only
- **Screenshot Retention:** Auto-delete after 30 days (configurable)
- **Log Scrubbing:** Remove potential PII from console logs

---

## 13. Success Metrics & KPIs

### 13.1 Functional Metrics
- **Accuracy:** 80%+ agreement with human QA on pass/fail
- **Coverage:** Successfully test 95%+ of valid game URLs
- **Precision:** <5% false positives (mark working games as broken)
- **Recall:** <10% false negatives (miss broken games)

### 13.2 Performance Metrics
- **P50 Execution Time:** <90 seconds per test
- **P95 Execution Time:** <180 seconds per test
- **Error Rate:** <5% infrastructure failures
- **Retry Success:** 80%+ of retries succeed

### 13.3 Business Metrics
- **QA Cycle Time:** Reduce from hours to minutes
- **Cost per Test:** <$0.05
- **Agent Improvement:** 10%+ increase in generated game quality (measured post-feedback loop integration)

---

## 14. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Agent loops infinitely | Medium | High | Max action count (20), total timeout (5min) |
| LLM gives inconsistent results | Medium | Medium | Structured prompts, confidence thresholds, heuristic fallback |
| Games require human verification (CAPTCHAs) | Low | Medium | Detect and skip, log for manual review |
| API costs exceed budget | Low | Medium | Aggressive caching, model selection, usage alerts |
| Browser memory leaks | Medium | High | Restart Chrome between tests, monitor memory |
| False negatives on complex games | High | Medium | Conservative pass threshold, human review queue |
| Scope creep | High | High | Strict 5-day core timeline, no stretch until complete |

---

## 15. Future Enhancements (Post-MVP)

### Phase 2: Intelligent Agent
- **Adaptive Interactions:** LLM suggests next actions based on game state
- **Gameplay Understanding:** Extract game rules and objectives
- **Comparative Analysis:** Compare game against similar titles
- **Regression Testing:** Detect changes between game versions

### Phase 3: Advanced Analytics
- **Performance Profiling:** FPS, memory usage, network waterfall
- **Accessibility Audit:** Color contrast, keyboard navigation
- **Cross-Browser Testing:** Test on Firefox, Safari
- **Mobile Emulation:** Responsive design validation

### Phase 4: Production Integration
- **CI/CD Pipeline:** Auto-test on every game generation
- **Feedback Loop:** Auto-retrain game builder based on QA results
- **Quality Dashboard:** Real-time monitoring of generated games
- **A/B Testing:** Compare game variants automatically

---

## 16. Appendices

### Appendix A: Sample Prompts

**Initial Evaluation Prompt:**
```
You are a QA engineer evaluating a browser game. I will provide:
1. Screenshots from different points in the test
2. Console logs from the browser
3. A list of interactions performed

Your task is to assess game quality across three dimensions:

LOAD SUCCESS (0-100): Did the game load and render correctly?
- 90-100: Perfect load, no issues
- 70-89: Minor delays or warnings
- 40-69: Significant load problems
- 0-39: Failed to load or blank screen

CONTROL RESPONSIVENESS (0-100): Do the controls work as expected?
- 90-100: Immediate, smooth response to all inputs
- 70-89: Responsive but minor lag or issues
- 40-69: Controls partially work
- 0-39: Controls unresponsive or broken

STABILITY (0-100): Did the game run without crashes or errors?
- 90-100: No errors, smooth execution
- 70-89: Minor warnings in console
- 40-69: Errors present but game still playable
- 0-39: Crashes, freezes, or critical errors

Respond ONLY with valid JSON in this exact format:
{
  "load_success": {
    "score": 85,
    "reasoning": "Game loaded quickly with minor asset delay"
  },
  ...
}
```

### Appendix B: Example Test Report

See: `/test/fixtures/example-report.json`

### Appendix C: Chrome DevTools Protocol Reference

Key CDP domains used:
- **Page:** Navigation, screenshots
- **Runtime:** Console logs, JavaScript execution
- **Input:** Keyboard, mouse simulation
- **DOM:** Element inspection

### Appendix D: Glossary

- **Playability Score:** Composite metric (0-100) measuring game quality
- **Confidence Score:** LLM's certainty in its evaluation (0-100)
- **Evidence:** Screenshots + logs collected during test
- **Interaction Strategy:** Predefined sequence of user actions
- **Pass Threshold:** Minimum score for game to pass QA (default: 70)

---

## 17. Contact & Support

**Project Owner:** Matt Smith  
**Email:** matt.smith@superbuilders.school  
**Slack:** @matt.smith  

**Repository:** github.com/dreamup/qa-agent (TBD)  
**Documentation:** /docs  
**Issue Tracker:** GitHub Issues  

---

**Document Version History:**
- v2.0 (2025-11-03): Detailed PRD with Go implementation
- v1.1 (2025-11-03): Initial project specification
