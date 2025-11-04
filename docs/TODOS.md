# DreamUp QA Agent - TODO List

## Current Sprint: Gameplay Interaction Fixes

### Phase 1: Fix Input System (CRITICAL - ~30 min)
- [ ] Add `FocusGameCanvas()` method to `internal/agent/ui_detection.go`
  - Set tabindex on canvas via JavaScript
  - Explicitly focus the canvas element
  - Verify focus was successful

- [ ] Add `SendKeyboardEventToCanvas()` method to `internal/agent/ui_detection.go`
  - Use JavaScript dispatchEvent for keyboard events
  - Send both keydown and keyup events
  - Dispatch to both canvas and window for compatibility

- [ ] Add `WaitForGameReady()` method to `internal/agent/ui_detection.go`
  - Check if canvas has been rendered (not blank)
  - Poll until ready or timeout (10s max)
  - Return true/false for success

- [ ] Update gameplay loop in `cmd/server/main.go` (lines 367-408)
  - Call `FocusGameCanvas()` before sending inputs
  - Replace `chromedp.KeyEvent()` with `SendKeyboardEventToCanvas()`
  - Replace fixed `time.Sleep(3s)` with `WaitForGameReady()`
  - Improve logging to show what's happening

### Phase 2: Add Verification (HIGH - ~20 min)
- [ ] Create `internal/agent/verification.go`
  - Add `VerifyScreenChanged()` - pixel diff comparison
  - Add `VerifyGameResponding()` - check if inputs cause visual changes
  - Add helper `absDiff()` for uint32 comparison

- [ ] Integrate verification into gameplay loop
  - Capture screenshot before gameplay starts
  - After 2s of inputs, verify screen has changed
  - Log warning if game appears frozen/unresponsive
  - Continue anyway (don't fail the test)

### Phase 3: Better Timing & Polish (MEDIUM - ~10 min)
- [ ] Increase gameplay duration from 10s to 15-20s
- [ ] Add progress updates during gameplay loop
- [ ] Better error handling and fallback logic
- [ ] Clean up logging messages

### Phase 4: Vision-Based AI (OPTIONAL - Future)
- [ ] Research LLM vision API integration
- [ ] Design prompt for gameplay decision making
- [ ] Implement adaptive input selection
- [ ] Test with different game types

---

## UI/UX Polish (Lower Priority)

### Frontend Styling
- [x] Style test status page with Tailwind
- [ ] Style report view page with Tailwind
- [ ] Add loading skeletons
- [ ] Improve error messages

### Backend
- [ ] Add persistent storage (Redis/PostgreSQL)
- [ ] Implement test history pagination
- [ ] Add filtering/sorting options
- [ ] API rate limiting

---

## Technical Debt

### Testing
- [ ] Add unit tests for UI detection
- [ ] Add integration tests for gameplay
- [ ] Test with various game types
- [ ] Performance benchmarking

### Documentation
- [ ] API documentation
- [ ] Deployment guide
- [ ] Architecture diagrams
- [ ] Contributing guidelines

### Infrastructure
- [ ] Docker containerization
- [ ] CI/CD pipeline
- [ ] Monitoring and alerting
- [ ] Log aggregation

---

## Completed
- [x] Install Tailwind CSS v4 and dependencies
- [x] Configure Vite plugin for Tailwind
- [x] Update CSS to import Tailwind
- [x] Refactor UI components to use Tailwind classes
- [x] Fix gameplay interaction - click play button and interact with canvas
- [x] Improve AI evaluation with 10s of actual gameplay
- [x] Rebuild and restart server with gameplay improvements
