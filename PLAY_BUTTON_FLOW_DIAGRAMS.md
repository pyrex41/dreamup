# Game Initialization Flow Diagram & Visual Architecture

## Complete Test Execution Flow (ASCII Diagram)

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Test Start (runTest)                            │
│                        cmd/qa/test.go                                │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │ Browser Manager     │
                    │ Creation            │
                    │ browser.go:64       │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │ Console Logger      │
                    │ StartCapture()      │
                    │ evidence.go:72      │
                    └──────────┬──────────┘
                               │
                 ┌─────────────▼─────────────┐
                 │  GAME URL LOADING STAGE   │
                 │                           │
                 │  bm.LoadGame(testURL)     │
                 │  browser.go:79            │
                 │  • Navigate to URL        │
                 │  • WaitReady("body")      │
                 │  • Timeout: 45 seconds    │
                 │  • Waits: DOMContentLoaded│
                 └─────────────┬─────────────┘
                               │
                    ┌──────────▼──────────┐
                    │ Initial Screenshot  │
                    │ Capture             │
                    │ test.go:85          │
                    │ (1280x720)          │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │ Page Load Wait      │
                    │ 2 seconds           │
                    │ test.go:97          │
                    └──────────┬──────────┘
                               │
        ┌──────────────────────┴──────────────────────┐
        │                                             │
     ┌──▼────────────────────┐    ┌─────────────────▼──────────────────┐
     │ COOKIE CONSENT STAGE  │    │     PLAY BUTTON STAGE              │
     │                       │    │  ← YOUR FOCUS AREA                 │
     │ AcceptCookieConsent() │    │ ClickStartButton()                 │
     │ ui_detection.go:349   │    │ ui_detection.go:270                │
     │ • Find accept buttons │    │                                    │
     │ • Text patterns       │    │ Step 1: Find buttons by text       │
     │ • Click if visible    │    │ ─────────────────────────────────  │
     │ • Timeout: <1s        │    │ document.querySelectorAll()        │
     │ Returns: clicked?     │    │ button, a[role="button"],          │
     │                       │    │ div[role="button"], etc.           │
     │ test.go:103-113       │    │                                    │
     └──────────┬────────────┘    │ Step 2: Check text patterns       │
                │                 │ ─────────────────────────────────  │
                │                 │ text === 'play'                    │
                │                 │ text === 'start'                   │
                │                 │ text === 'begin'                   │
                │                 │ text === 'play game'               │
                │                 │ text === 'start game'              │
                │                 │ text.includes('play now')          │
                │                 │ text.includes('start now')         │
                │                 │                                    │
                │                 │ Step 3: Check visibility           │
                │                 │ ─────────────────────────────────  │
                │                 │ if (btn.offsetParent !== null)     │
                │                 │                                    │
                │                 │ Step 4: Click button               │
                │                 │ ─────────────────────────────────  │
                │                 │ btn.click()                        │
                │                 │ return true                        │
                │                 │                                    │
                │                 │ Step 5: Fallback to canvas         │
                │                 │ ─────────────────────────────────  │
                │                 │ if (no button found) {             │
                │                 │   canvas.click()                   │
                │                 │ }                                  │
                │                 │                                    │
                │                 │ test.go:117-127                    │
                └──────────┬───────┴─────────────────────────────────┘
                           │
                ┌──────────▼──────────┐
                │ Game Initialization │
                │ Wait                │
                │ 500ms (or 2s)       │
                │ test.go:124         │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ Keyboard Input      │
                │ Simulation          │
                │ test.go:134-151     │
                │ • ArrowUp (200ms)   │
                │ • ArrowDown (200ms) │
                │ • Space (200ms)     │
                │ • ArrowLeft (200ms) │
                │ • ArrowRight (200ms)│
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ Final Screenshot    │
                │ Capture             │
                │ test.go:159         │
                │ (1280x720)          │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ Console Logs Save   │
                │ SaveToTemp()        │
                │ evidence.go:243     │
                │ (JSON file)         │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ AI Evaluation       │
                │ evaluator.go:189    │
                │ GPT-4 Vision        │
                │ Timeout: ~5-10s     │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ Report Generation   │
                │ Build & Save        │
                │ test.go:242-252     │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ S3 Upload (Optional)│
                │ reporter.go         │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │ Test Complete       │
                │ Summary Display     │
                │ test.go:270-283     │
                └──────────────────────┘
```

---

## Play Button Detection Logic (Detailed Flow)

```
ClickStartButton()
│
├─ Get all potential buttons from DOM
│  └─ document.querySelectorAll([
│     'button',
│     'a[role="button"]',
│     'div[role="button"]',
│     'a',
│     'span[role="button"]',
│     'input[type="button"]',
│     'input[type="submit"]'
│  ])
│
├─ FOR EACH button found:
│  │
│  ├─ Extract text:
│  │  └─ text = btn.textContent.toLowerCase().trim()
│  │
│  ├─ Check text patterns (FIRST MATCH WINS):
│  │  │
│  │  ├─ text === 'play'           ? CLICK ✓
│  │  ├─ text === 'start'          ? CLICK ✓
│  │  ├─ text === 'begin'          ? CLICK ✓
│  │  ├─ text === 'play game'      ? CLICK ✓
│  │  ├─ text === 'start game'     ? CLICK ✓
│  │  ├─ text.includes('play now') ? CLICK ✓
│  │  └─ text.includes('start now')? CLICK ✓
│  │
│  ├─ Check value attribute:
│  │  ├─ value === 'play'          ? CLICK ✓
│  │  └─ value === 'start'         ? CLICK ✓
│  │
│  └─ Check visibility:
│     └─ if (btn.offsetParent !== null) {
│          btn.click()
│          return true;
│        }
│
├─ IF no button matched (FALLBACK):
│  │
│  ├─ Look for canvas element
│  ├─ Check if visible
│  └─ If found: canvas.click()
│
└─ RETURN:
   ├─ true  = button was clicked
   └─ false = no button/canvas found
```

---

## Console Logging Capture Timeline

```
Time    Event                          Console Logger State
────────────────────────────────────────────────────────────
 0ms    ConsoleLogger created          ✓ Listener registered
        StartCapture() called           ✓ Runtime.Enable() called
        
10ms    Browser starts
        
100ms   Game page loads
        console.log("Starting...")     ✓ CAPTURED
        
150ms   Initial screenshot captured
        
1150ms  Page load wait (2s) ends
        
1200ms  Cookie consent checked
        User clicks accept
        console.log("Cookies OK")      ✓ CAPTURED
        
1400ms  Play button check
        User clicks start
        console.log("Game start")      ✓ CAPTURED
        
1900ms  Game initialization wait (500ms)
        console.warn("Missing asset")  ✓ CAPTURED
        
2100ms  Keyboard events sent
        console.error("Bad input")     ✓ CAPTURED
        
2400ms  Final screenshot
        
2500ms  Game state updates
        console.log("Score: 100")      ✓ CAPTURED
        
2600ms  Console logger stops
        SaveToTemp() called
        
2650ms  Logs written to JSON file
```

**Important:** All logs from step 10ms onwards are captured, including logs from page load, button clicks, and gameplay!

---

## Code Path for Play Button Click

### Entry Point → Execution → Result

```
test.go:117
│
└─ detector.ClickStartButton()
   │
   └─ ui_detection.go:270
      │
      ├─ Prepare JavaScript script
      │  └─ ui_detection.go:272-301
      │
      ├─ Execute in browser context
      │  └─ chromedp.Evaluate(script, &clicked)
      │
      ├─ JavaScript runs in browser
      │  └─ Searches DOM for matching buttons
      │  └─ Checks visibility
      │  └─ Clicks if found
      │  └─ Returns boolean
      │
      └─ Return result
         │
         └─ test.go:121-127
            │
            ├─ if clicked = true
            │  └─ fmt.Println("✅ Game started!")
            │  └─ time.Sleep(500ms)
            │
            └─ else (clicked = false)
               └─ fmt.Println("No start button detected...")
```

---

## Directory Structure (Relevant Files)

```
dreamup/
├── cmd/
│   └── qa/
│       ├── main.go           (Entry point, CLI setup)
│       ├── test.go           ← MAIN TEST ORCHESTRATION
│       └── config.go
│
├── internal/
│   ├── agent/
│   │   ├── browser.go        ← URL LOADING (LoadGame)
│   │   ├── ui_detection.go   ← PLAY BUTTON DETECTION
│   │   ├── evidence.go       ← CONSOLE LOGGING & SCREENSHOTS
│   │   ├── interactions.go
│   │   ├── vision.go
│   │   ├── vision_dom.go
│   │   ├── video.go
│   │   └── errors.go
│   │
│   ├── evaluator/
│   │   └── llm.go            (AI evaluation)
│   │
│   └── reporter/
│       ├── report.go         (Report generation)
│       └── s3.go
│
├── ARCHITECTURE.md           (Overall system design)
├── PLAY_BUTTON_INTEGRATION.md (You are here!)
└── PLAY_BUTTON_QUICK_REFERENCE.md
```

---

## Signal Flow for Play Button Click

```
Browser Context (chromedp)
│
├─ Game loaded
├─ DOM ready
├─ Initial screenshot taken
├─ Page load wait (2s)
├─ Cookie consent (if any)
│
└─ ClickStartButton() called
   │
   ├─ chromedp.Evaluate() sends JavaScript to browser
   │  │
   │  └─ JavaScript executes in browser sandbox
   │     │
   │     ├─ Query DOM for buttons
   │     ├─ Check text patterns
   │     ├─ Verify visibility
   │     └─ Click if match found
   │
   └─ Boolean result returned to Go
      │
      ├─ true  → Game started (wait 500ms)
      └─ false → No button (continue anyway)

Next phase: Keyboard input simulation
```

---

## Error Handling & Recovery

```
If ClickStartButton() errors:
│
├─ Error caught in test.go:117
│  └─ if clicked, err := detector.ClickStartButton()
│
├─ Error is logged as warning (not test failure)
│  └─ "Start button check failed: <error>"
│
├─ Test continues anyway
│  └─ Game might auto-start
│  └─ Or it might be playable without clicking
│
└─ Result saved to report
   └─ reportBuilder.AddMetadata("ui_warning_1", warning)
```

**Key:** Play button click failure does NOT stop the test. Tests continue regardless.

---

## Related Operations (Before/After Play Button)

```
BEFORE Play Button Click:
├─ Browser navigation (45s timeout)
├─ Screenshot #1 (initial state)
├─ 2-second wait
├─ Cookie consent handling
└─ ← PLAY BUTTON CLICK HAPPENS HERE

AFTER Play Button Click:
├─ 500ms wait for game UI animation
├─ Keyboard input simulation (1s total)
├─ Screenshot #2 (final state)
├─ Console log save
├─ AI evaluation (5-10s)
└─ Report generation
```

