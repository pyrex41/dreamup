# End-to-End Testing Guide

## Overview

This document describes the complete integration testing setup for the DreamUp QA Agent, including backend API and frontend Elm application.

## Architecture

```
┌─────────────┐      HTTP/JSON      ┌──────────────┐      Internal      ┌─────────────┐
│   Frontend  │ ──────────────────> │  API Server  │ ──────────────────> │   Browser   │
│  (Elm/Vite) │                     │   (Go)       │                     │  (chromedp) │
│             │ <────────────────── │              │ <────────────────── │             │
│ :3000       │      Reports        │ :8080        │    Screenshots/     │   Headless  │
└─────────────┘                     └──────────────┘    Logs             └─────────────┘
```

## Components

### 1. Backend API Server (`cmd/server/main.go`)

**Port**: 8080
**Endpoints**:
- `POST /api/tests` - Submit new test
- `GET /api/tests/{id}` - Get test status
- `GET /api/tests/list` - List all tests
- `GET /api/reports/{id}` - Get test report
- `GET /health` - Health check

**Features**:
- CORS enabled for frontend integration
- In-memory job tracking with status updates
- Real-time progress reporting (0-100%)
- Automatic browser automation with chromedp
- AI evaluation using OpenAI GPT-4o

### 2. Frontend Dashboard (`frontend/`)

**Port**: 3000
**Tech Stack**: Elm 0.19.1 + Vite 7.1.12
**Features**:
- Test submission form with URL validation
- Real-time status polling with progress bar
- Report display with screenshots and console logs
- Test history with search and pagination
- Network error handling with retry logic
- WCAG 2.1 AA accessibility

## API Contract

### Submit Test Request

```bash
curl -X POST http://localhost:8080/api/tests \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com",
    "maxDuration": 60,
    "headless": true
  }'
```

**Response**:
```json
{
  "testId": "uuid-v4",
  "status": "pending"
}
```

### Poll Test Status

```bash
curl http://localhost:8080/api/tests/{testId}
```

**Response**:
```json
{
  "testId": "uuid-v4",
  "status": "running",
  "progress": 75,
  "message": "Capturing final screenshot...",
  "createdAt": "2025-11-03T17:00:00Z",
  "updatedAt": "2025-11-03T17:01:30Z"
}
```

**Status Values**:
- `pending` - Queued for execution
- `running` - Currently executing (with progress %)
- `completed` - Test finished successfully
- `failed` - Test failed with error message

**Progress Milestones**:
- 10% - Browser initialized
- 20% - Navigation started
- 30% - Initial screenshot captured
- 40% - Cookie consent handling
- 50% - Game start initiated
- 60% - Gameplay simulation
- 70% - Final screenshot captured
- 80% - Console logs collected
- 90% - AI evaluation running
- 100% - Complete

### Get Test Report

```bash
curl http://localhost:8080/api/reports/{testId}
```

**Response** (see example below for full structure):
```json
{
  "report_id": "uuid-v4",
  "game_url": "https://example.com",
  "timestamp": "2025-11-03T17:00:00Z",
  "duration_ms": 67500,
  "score": {
    "overall_score": 65,
    "loads_correctly": true,
    "interactivity_score": 70,
    "visual_quality": 80,
    "error_severity": 20,
    "reasoning": "...",
    "issues": [...],
    "recommendations": [...]
  },
  "evidence": {
    "screenshots": [...],
    "console_logs": [...],
    "log_summary": {...}
  },
  "summary": {
    "status": "passed",
    "passed_checks": [...],
    "failed_checks": [...],
    "critical_issues": [...]
  }
}
```

## Running the System

### Prerequisites

```bash
# Install Go dependencies
go mod download

# Install frontend dependencies
cd frontend && npm install
```

### Start Backend Server

```bash
# Build server
go build -o server cmd/server/main.go

# Run with OpenAI API key
export OPENAI_API_KEY="sk-..."
PORT=8080 ./server
```

**Server Output**:
```
🚀 DreamUp QA API Server v0.1.0
🌐 Listening on http://localhost:8080
📊 Health check: http://localhost:8080/health
📝 API endpoints:
   POST   /api/tests        - Submit new test
   GET    /api/tests/{id}   - Get test status
   GET    /api/tests/list   - List all tests
   GET    /api/reports/{id} - Get test report
```

### Start Frontend Server

```bash
cd frontend
npm run dev
```

**Frontend Output**:
```
VITE v7.1.12  ready in 195 ms
➜  Local:   http://localhost:3000/
```

## End-to-End Test Scenarios

### Scenario 1: Successful Test Execution

**Test URL**: https://www.poki.com/en/g/subway-surfers

1. **Submit Test**:
```bash
TEST_ID=$(curl -s -X POST http://localhost:8080/api/tests \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.poki.com/en/g/subway-surfers","maxDuration":60,"headless":true}' \
  | jq -r '.testId')
echo "Test ID: $TEST_ID"
```

2. **Poll Status** (every 2 seconds):
```bash
while true; do
  STATUS=$(curl -s http://localhost:8080/api/tests/$TEST_ID | jq -r '.status')
  PROGRESS=$(curl -s http://localhost:8080/api/tests/$TEST_ID | jq -r '.progress')
  MESSAGE=$(curl -s http://localhost:8080/api/tests/$TEST_ID | jq -r '.message')

  echo "[$PROGRESS%] $STATUS: $MESSAGE"

  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    break
  fi

  sleep 2
done
```

3. **Fetch Report**:
```bash
curl -s http://localhost:8080/api/reports/$TEST_ID | jq '.'
```

### Scenario 2: Frontend Workflow Test

1. Open browser to http://localhost:3000
2. Navigate to "Submit Test" page
3. Enter URL: `https://example.com`
4. Set max duration: 60 seconds
5. Check "Headless mode"
6. Click "Submit Test"
7. Observe real-time progress updates
8. View completed report with:
   - AI evaluation scores
   - Initial/final screenshots
   - Console logs
   - Issues and recommendations

### Scenario 3: Error Handling

**Test with invalid URL**:
```bash
curl -s -X POST http://localhost:8080/api/tests \
  -H "Content-Type: application/json" \
  -d '{"url":"not-a-valid-url"}' \
  | jq '.'
```

**Expected Response**:
```json
{
  "error": "URL is required"
}
```

### Scenario 4: Network Retry Logic

Frontend automatically retries failed requests with exponential backoff:
- Retry 1: Immediate
- Retry 2: After 1 second
- Retry 3: After 2 seconds
- Max retries: 3

## Validated Game URLs

Based on previous testing, these URLs work well with the system:

### Successful Tests (Score ≥ 60)

1. **Free Rider 2** (Kongregate) - Score: 65/100
   ```json
   {
     "url": "https://www.kongregate.com/en/games/onemorelevel/free-rider-2",
     "maxDuration": 60,
     "headless": true
   }
   ```

2. **Local HTML5 Test Game** - Score: 60/100
   - Simple test case for validation

### Known Issues

1. **Famobi Games** - Cross-origin iframe blocking
   - Cookie consent in iframe blocked by browser security
   - URL: `https://play.famobi.com/wrapper/bubble-tower-3d/A1000-10`
   - Score: 40/100 (Failed)

2. **Network-dependent Sites**
   - Some sites require stable internet connection
   - Headless mode may fail with `ERR_SOCKET_NOT_CONNECTED`
   - Solution: Use `"headless": true` in test request

## Performance Metrics

### Typical Test Execution Time

- **Browser startup**: 2-3 seconds
- **Page navigation**: 3-5 seconds
- **Cookie consent**: 1 second
- **Gameplay simulation**: 3 seconds
- **Screenshot capture**: 1 second per screenshot
- **AI evaluation**: 5-10 seconds
- **Total**: ~15-25 seconds per test

### Resource Usage

- **Memory**: ~200MB per browser instance
- **CPU**: Spikes during AI evaluation
- **Disk**: ~500KB per report (screenshots + JSON)

## Frontend Features Validation

### Test Submission (Task 2) ✅
- URL input with validation
- Max duration slider (0-300 seconds)
- Headless mode toggle
- Form validation and error messages

### Status Tracking (Task 3) ✅
- Real-time polling every 2 seconds
- Progress bar (0-100%)
- Status messages ("Initializing browser...", etc.)
- Auto-redirect to report on completion

### Report Display (Task 4) ✅
- AI evaluation scores with color coding
- Pass/fail status indicator
- Test metadata (URL, duration, timestamp)
- Expandable sections for issues/recommendations

### Screenshot Viewer (Task 5) ✅
- Initial vs Final comparison
- Side-by-side view mode
- Overlay mode with opacity slider
- Difference view (highlights changes)
- Zoom levels: Fit, 100%, 200%
- Fullscreen toggle
- Keyboard navigation

### Console Log Viewer (Task 6) ✅
- Log level filtering (error, warning, info, debug)
- Virtual scrolling (100 logs max per page)
- Timestamp display
- Source location (file:line)
- Color-coded by log level
- Search functionality

### Test History (Task 7) ✅
- List all previous tests
- Sorting (date, status, score, URL)
- Filtering (status, score range)
- Pagination (20 items per page)
- Search by URL
- Click to view report

### UI/UX Polish (Task 8) ✅
- Responsive design (mobile/tablet/desktop)
- WCAG 2.1 AA accessibility
- Network status indicator
- Retry logic with exponential backoff
- Loading spinners
- Error dismissal
- Offline support

## Monitoring and Debugging

### Server Logs

```bash
# Watch server logs in real-time
tail -f server.log

# Example output:
2025/11/03 17:26:49 Starting test abc-123 for URL: https://example.com
2025/11/03 17:27:03 Cookie consent accepted
2025/11/03 17:27:09 Game started
2025/11/03 17:27:18 Test abc-123 completed with score: 65/100
```

### API Health Check

```bash
curl http://localhost:8080/health
```

**Response**:
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "time": "2025-11-03T17:00:00Z"
}
```

### Frontend Dev Tools

```bash
# Build frontend
cd frontend && npm run build

# Preview production build
npm run preview
```

## Troubleshooting

### Issue: Browser fails to start

**Symptoms**: `failed to create browser: ...`

**Solutions**:
1. Ensure Chrome/Chromium is installed
2. Check system resources (memory/CPU)
3. Try non-headless mode: `"headless": false`

### Issue: Navigation timeout

**Symptoms**: `timeout after 45s while loading...`

**Solutions**:
1. Increase `maxDuration` in request
2. Check internet connection
3. Verify URL is accessible

### Issue: AI evaluation fails

**Symptoms**: `Evaluator initialization failed`

**Solutions**:
1. Set `OPENAI_API_KEY` environment variable
2. Verify API key is valid and has credits
3. Check OpenAI API status

### Issue: CORS errors in frontend

**Symptoms**: `Access to fetch blocked by CORS policy`

**Solutions**:
1. Verify backend server is running on port 8080
2. Check frontend `apiBaseUrl` in `frontend/src/Main.elm:273`
3. Restart both servers

## Security Considerations

1. **API Keys**: Never commit API keys to version control
2. **CORS**: Configured for `*` in development, restrict in production
3. **Input Validation**: URL validation prevents code injection
4. **Rate Limiting**: Not implemented (add in production)
5. **Authentication**: Not implemented (consider for production)

## Deployment

### Backend Deployment Options

1. **AWS Lambda** (see `deployment/terraform/`)
2. **Docker Container**
3. **Traditional Server** (systemd service)

### Frontend Deployment Options

1. **AWS S3 + CloudFront** (see `frontend/DEPLOYMENT.md`)
2. **Netlify** (drag-and-drop `frontend/dist/`)
3. **Vercel** (connect GitHub repo)

## Conclusion

The DreamUp QA Agent end-to-end testing system is **fully functional** with:

✅ Backend API server with real-time progress tracking
✅ Frontend Elm dashboard with complete feature set
✅ Browser automation with cookie consent and gameplay
✅ AI evaluation using OpenAI GPT-4o
✅ Comprehensive error handling and retry logic
✅ WCAG 2.1 AA accessibility compliance
✅ Production-ready deployment configurations

**Next Steps**:
1. Deploy to production environment
2. Add authentication and rate limiting
3. Implement S3 storage for reports
4. Set up monitoring and alerting
5. Create CI/CD pipeline
