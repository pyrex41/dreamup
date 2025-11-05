# Quick Reference: Game Initialization & Play Button Handling

## File Locations & Code References

### 1. Main Test Flow Orchestration
**File:** `/Users/reuben/gauntlet/dreamup/cmd/qa/test.go`

Key sections:
- **Lines 43-287:** `runTest()` function - main test orchestration
- **Line 79:** Game loading with 45-second timeout
- **Lines 83-93:** Initial screenshot capture
- **Lines 100-113:** Cookie consent handling
- **Lines 115-127:** Play button click attempt
- **Lines 130-131:** 2-second wait for gameplay
- **Lines 134-151:** Keyboard interactions simulation

### 2. Browser Management & URL Loading
**File:** `/Users/reuben/gauntlet/dreamup/internal/agent/browser.go`

Key functions:
- **Lines 114-118:** `LoadGame()` - 45-second timeout navigation
- **Lines 94-112:** `NavigateWithTimeout()` - timeout wrapper
- **Lines 82-92:** `Navigate()` - basic navigation with DOMContentLoaded wait

### 3. UI Detection & Play Button Logic
**File:** `/Users/reuben/gauntlet/dreamup/internal/agent/ui_detection.go`

Key sections:
- **Lines 56-74:** `StartButtonPattern` definition
- **Lines 119-157:** `CookieConsentPattern` definition
- **Lines 268-313:** `ClickStartButton()` - main play button detection
- **Lines 347-434:** `AcceptCookieConsent()` - reference implementation
- **Lines 437-522:** `FocusGameCanvas()` - canvas focus logic
- **Lines 524-604:** `SendKeyboardEventToCanvas()` - keyboard input
- **Lines 606-675:** `SendKeyboardEventToWindow()` - DOM game input
- **Lines 677-749:** `WaitForGameReady()` - canvas readiness polling

### 4. Console Logging
**File:** `/Users/reuben/gauntlet/dreamup/internal/agent/evidence.go`

Key sections:
- **Lines 122-144:** `ConsoleLogger` structure definition
- **Lines 146-164:** `StartCapture()` - initialize console listener
- **Lines 167-210:** `handleConsoleEvent()` - process console events
- **Lines 242-257:** `SaveToTemp()` - save logs to file

---

## Current Play Button Detection Logic

### JavaScript Pattern (ClickStartButton - Lines 272-301)

The function tries to:

1. **Find buttons by text content:**
   - Exact matches: "play", "start", "begin", "play game", "start game"
   - Substring matches: "play now", "start now"
   
2. **Check visibility:**
   - Uses `offsetParent !== null` visibility check

3. **Fallback:**
   - Clicks canvas element if no button found

### Supported Element Types

```javascript
document.querySelectorAll('button, a[role="button"], 
                          div[role="button"], a, 
                          span[role="button"], 
                          input[type="button"], 
                          input[type="submit"]')
```

---

## Test Execution Timeline

| Step | File:Lines | What Happens | Duration |
|------|-----------|--------------|----------|
| 1 | test.go:64 | Browser created | N/A |
| 2 | test.go:72 | Console logger started | N/A |
| 3 | test.go:79 | Game URL loaded | ~1-45s |
| 4 | test.go:85 | Initial screenshot | ~0.5s |
| 5 | test.go:97 | Wait for page load | 2s |
| 6 | test.go:103 | Cookie consent check | <1s |
| 7 | **test.go:117** | **Play button click** | **<1s** |
| 8 | test.go:131 | Wait for gameplay | 2s |
| 9 | test.go:147 | Send keyboard inputs | ~1s total |
| 10 | test.go:159 | Final screenshot | ~0.5s |
| 11 | test.go:171 | Save console logs | <1s |
| 12 | test.go:189 | AI evaluation | ~5-10s |

**Total typical test duration:** ~15-20 seconds for initialization, ~30-60s for full test

---

## Console Logging Behavior

**When is console logging captured?**

```
Console logger listener enabled
         ↓
Game page loads
         ↓
Initial screenshot (logs from page load are captured)
         ↓
Wait 2 seconds (logs captured)
         ↓
Cookie consent click (logs captured)
         ↓
Play button click (logs captured) ← LOGS FROM THIS ARE CAPTURED
         ↓
Final screenshot
         ↓
Logs saved to file
```

**Important:** All console.log/warn/error messages from page load onwards are automatically captured and saved to a JSON file.

---

## Extending Play Button Detection

### Quick Fix: Add More Text Patterns

**File:** `/Users/reuben/gauntlet/dreamup/internal/agent/ui_detection.go`
**Lines:** 280-283 (modify the if condition)

Current patterns:
```javascript
if (text === 'play' || text === 'start' || text === 'begin' ||
    text === 'play game' || text === 'start game' ||
    text.includes('play now') || text.includes('start now') ||
    value === 'play' || value === 'start')
```

Add new patterns like:
```javascript
text.includes('let\'s go') ||
text.includes('begin adventure') ||
text.includes('get started') ||
text === 'go' ||
text.includes('launch') ||
text.includes('start game') ||
text.includes('press to play')
```

### Advanced: Create New Detection Method

**File:** `/Users/reuben/gauntlet/dreamup/internal/agent/ui_detection.go`

Create a new method in `UIDetector` struct:

```go
func (d *UIDetector) ClickPlayButtonWithRetry() (bool, error) {
    // Try multiple times with backoff
    // Implement timeout/retry logic
    // Log which button was clicked
}
```

Then modify `test.go` line 117 to call your new method.

---

## Error Handling

If play button click fails:

```go
// test.go lines 117-120
if clicked, err := detector.ClickStartButton(); err != nil {
    warning := fmt.Sprintf("Start button check failed: %v", err)
    uiWarnings = append(uiWarnings, warning)
    fmt.Printf("   ⚠️  Warning: %s\n", warning)
}
```

The test continues anyway (doesn't fail if button not found) because games may auto-start.

---

## Related Files (Reference)

- `/Users/reuben/gauntlet/dreamup/internal/agent/browser.go` - Browser lifecycle & navigation
- `/Users/reuben/gauntlet/dreamup/internal/agent/evidence.go` - Screenshot & logging
- `/Users/reuben/gauntlet/dreamup/internal/agent/interactions.go` - Game interactions
- `/Users/reuben/gauntlet/dreamup/internal/evaluator/llm.go` - AI evaluation
- `/Users/reuben/gauntlet/dreamup/internal/reporter/report.go` - Report generation

---

## Console Log Output Example

When the test runs, console logs are captured and saved to a JSON file with structure:

```json
[
  {
    "level": "log",
    "message": "Game initialized",
    "timestamp": "2024-11-04T12:34:56Z",
    "source": "https://example.com/game.js:42:15",
    "args": []
  },
  {
    "level": "error",
    "message": "Missing texture asset",
    "timestamp": "2024-11-04T12:34:57Z",
    "source": "https://example.com/game.js:156:8",
    "args": []
  }
]
```

The file is saved to the system temp directory and referenced in the test report.

---

## Key Integration Points

If you're enhancing play button handling:

1. **For new text patterns:** Edit `ClickStartButton()` JavaScript section (ui_detection.go:280)
2. **For retry logic:** Create new method in UIDetector (ui_detection.go)
3. **For AI-based detection:** Create button_detector.go in internal/agent/
4. **For test flow changes:** Modify runTest() in test.go (around lines 115-127)
5. **For logging:** Use fmt.Println() in test.go for user-facing output, console logs are auto-captured

---

## Testing Your Changes

```bash
# Test a specific game URL
cd /Users/reuben/gauntlet/dreamup
./qa test --url "https://example.com/game" --headless=false

# With custom timeout (default 45s per game)
./qa test --url "https://example.com/game" --headless=false --max-duration 300
```

Output will show:
- ✅ Game started! (if play button was clicked)
- No start button detected, game may auto-start (if no button found)
- Console logs saved to temp file (logged at end of test)

