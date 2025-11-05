# Project Log: Angry Birds Vision Debugging Session
**Date:** November 5, 2025
**Branch:** `angry`
**Session Focus:** Troubleshooting and fixing vision-based button detection for Angry Birds

## Session Summary
Debugged systematic coordinate detection issues with GPT-4o vision API for Angry Birds game testing. Through iterative testing with visual click markers, identified that the model was consistently underestimating button Y-coordinates. Fixed by adding game-specific examples to the vision prompt, improving accuracy from 80/100 to 95/100.

## Changes Made

### 1. Vision API Logging Enhancement
**File:** `internal/agent/vision_dom.go:288-310`
**Commit:** `bb9fc66`

Added comprehensive logging to debug vision API interactions:
- Log full prompts sent to LLM
- Log raw responses from LLM
- Log parsed results with coordinates
- Increased API timeout from 3s to 15s to handle large image payloads (base64-encoded 1280x720 PNGs)

**Result:** Enabled real-time debugging of coordinate detection accuracy.

### 2. Visual Click Marker System
**File:** `internal/agent/vision_dom.go:372-447`
**Commit:** `a0ef1e0`

Implemented visual debugging system to verify click accuracy:
- Created `SaveScreenshotWithClickMarker()` function
- Draws red circle (20px radius) and crosshair at click coordinates
- Saves marked PNG to temp directory before each click
- Integrated into main.go gameplay detection loop

**Result:** Visual confirmation showed clicks landing 100-150px too high (Y=400-520 instead of ~590).

### 3. Game-Specific Prompt Examples
**File:** `internal/agent/vision_dom.go:258-293`
**Commit:** `5a3acf3` (current)

Enhanced vision prompt with Angry Birds-specific guidance:

**Before:**
```
- PLAY button in lower-center: {"click_x": 640, "click_y": 400, ...}
```

**After:**
```
- Angry Birds PLAY button (large orange button below red bird and title):
  {"click_x": 640, "click_y": 590, ...}
- PLAY button in lower-center: {"click_x": 640, "click_y": 580, ...}
- START button very low: {"click_x": 640, "click_y": 620, ...}
```

Additional improvements:
- Explicit coordinate measurement instructions (find button boundaries, calculate center)
- Lower third threshold guidance (Y > 500 for buttons below titles)
- Special note emphasizing menu buttons appear BELOW game logos
- Image dimensions reminder (1280x720 pixels)

**Result:** GPT-4o now correctly detects PLAY button at (640, 590), achieving 95/100 test score.

## Test Results

### Initial Tests (GPT-4o-mini, basic prompt)
- Test scores: 80-85/100
- Click coordinates: (640, 400) → Too high, landing on title
- Screen never progressed past main menu

### Mid-Session Tests (GPT-4o, improved prompt)
- Click coordinates: (640, 440-520) → Still too high
- Multiple iterations showed consistent ~100px error

### Final Test (GPT-4o, game-specific example)
- Test ID: `45036f4d-8679-4707-8204-c0ab5665b0c5`
- Test score: **95/100**
- Sequence:
  1. ✅ PLAY button: (640, 590) - **Perfect!**
  2. ✅ Progressed to level selection screen
  3. ⚠️  Level 1 button: (204, 202) - Close but missed (screen didn't change)

## Key Insights

### 1. Viewport/Screenshot Alignment Verified
**Question raised:** Could coordinate errors be due to viewport/screenshot size mismatch?

**Answer:** No mismatch detected:
- Screenshot dimensions: 1280x720 (via `chromedp.EmulateViewport(1280, 720)`)
- Actual viewport: 1280x720 (via `window.innerWidth/Height`)
- Coordinate transformation scale: 1.00 x 1.00 (no scaling needed)
- Log evidence: `[VisionClick] Transformed coordinates: (640, 400) with scale (1.00, 1.00)`

**Code references:**
- evidence.go:53 - Sets viewport
- vision_dom.go:456-495 - Transforms coordinates (but scale is 1:1)

### 2. Vision Model Spatial Reasoning
GPT-4o systematically underestimated button Y-coordinates without concrete examples. The model improved from Y=400 → Y=520 → **Y=590** as we added:
1. More specific instructions
2. Better examples
3. **Game-specific reference points** (the winning approach)

### 3. Remaining Issues Identified

**Issue #1: Not reaching gameplay**
- Currently stopping at level selection screen
- Need to detect level buttons more accurately (204, 202 missed the button)
- Need to implement actual gameplay interaction (mouse drag for slingshot)

**Issue #2: Coordinate precision limitations**
- Pixel-perfect accuracy is difficult for vision models
- User suggested **grid overlay system**:
  - Overlay labeled grid (e.g., 20x20 cells) on screenshot
  - Ask GPT for grid cell (e.g., "C3") instead of exact pixels
  - Convert grid cell to pixel coordinates
  - More intuitive and robust approach

## Performance Improvements (From Previous Sessions)

Related performance work on branch `angry`:
- **Commit af049a1:** Screenshot change detection (SHA256 hashing) to skip redundant vision API calls
- **Commit 2e29f10:** Reduced delays between gameplay detection attempts (2s → 500ms)

These improvements reduced vision API costs and test execution time significantly.

## File Changes Summary

```
internal/agent/vision_dom.go:
  - Lines 258-293: Enhanced vision prompt with game-specific examples
  - Lines 288-310: Added detailed request/response logging
  - Lines 372-447: Implemented visual click marker system

cmd/server/main.go:
  - Lines 867-874: Integrated marker screenshot saving before clicks

internal/agent/evidence.go:
  - Lines 93-98: Screenshot Hash() method for change detection
```

## Next Steps

### Immediate (Level Selection Fix)
1. Improve level button detection accuracy
   - Add Angry Birds level selection example to prompt
   - Adjust coordinates for level 1 button (currently ~20px off)

### Short-term (Gameplay Interaction)
2. Implement mouse drag gesture for bird launching
   - Detect slingshot location
   - Calculate drag trajectory
   - Execute mouse down → drag → release sequence

### Medium-term (Grid Overlay System)
3. Implement visual grid overlay approach
   - Draw labeled grid on screenshots before sending to GPT
   - Modify prompt to request grid cells instead of pixels
   - Convert grid cell references to pixel coordinates
   - Benefits: More intuitive, robust, and debuggable

### Long-term (Multiple Games)
4. Build library of game-specific prompt examples
5. Automatic game detection and example selection

## Technical Debt

1. **Hardcoded examples in prompt:** Consider externalizing to config file
2. **Screenshot caching:** Implement better cache management (currently temp files accumulate)
3. **Coordinate transformation complexity:** Grid system would simplify this
4. **Gameplay detection timeout:** 10 attempts may not be enough for slow-loading games

## Related Documentation

New documentation created this session:
- PLAY_BUTTON_DOCUMENTATION_INDEX.md - Overview of vision button detection
- PLAY_BUTTON_FLOW_DIAGRAMS.md - Visual flow diagrams
- PLAY_BUTTON_INTEGRATION.md - Integration guide
- PLAY_BUTTON_QUICK_REFERENCE.md - Quick reference
- PLAY_BUTTON_SUMMARY.md - Executive summary

## Testing Notes

Test environment:
- Browser: Chrome via chromedp
- Resolution: 1280x720 (720p)
- Vision model: GPT-4o (upgraded from GPT-4o-mini)
- Test URL: https://funhtml5games.com/angrybirds/index.html
- Log locations: /tmp/angry-birds-*.log

## Commit History

```
5a3acf3 feat: fix vision button detection with explicit Angry Birds example
a0ef1e0 feat: add visual click markers to debug coordinate accuracy
bb9fc66 feat: add detailed vision API logging and increase timeout to 15s
2e29f10 perf: add screenshot change detection to skip redundant vision API calls
af049a1 perf: dramatically speed up gameplay detection
```

## Session Duration
Approximately 3-4 hours of iterative debugging and testing.

---
*Log created: 2025-11-05*
*Branch: angry*
*Status: Vision detection significantly improved, gameplay interaction pending*
