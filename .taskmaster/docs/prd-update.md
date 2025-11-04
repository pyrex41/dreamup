# DreamUp QA Agent - PRD Update: DOM-Based Game Support

## Document Metadata
- **Version:** 1.0
- **Date:** November 3, 2025
- **Related PRD:** prd-init.md
- **Change Scope:** Add support for DreamUp DOM-based games while maintaining external HTML5 canvas game testing
- **Author:** Reuben (with Claude Code)

## Executive Summary

This update clarifies requirements for the DreamUp QA Agent based on feedback from Matt Smith. The original PRD (prd-init.md) assumed we would be testing generic HTML5 canvas-rendered games. However, **DreamUp's game engine uses DOM elements for UI**, not canvas rendering.

**Key Changes:**
1. Add dual-mode support: Auto-detect canvas games (external) vs DOM-based games (DreamUp)
2. Support input schema from game dev agent (JSON or natural language format)
3. Implement DOM-targeted event dispatch (not canvas-specific)
4. Maintain backward compatibility with existing canvas game tests

This update does NOT replace the original PRD - it extends it with DreamUp-specific requirements while preserving the ability to test external HTML5 games.

---

## 1. Background & Context

### 1.1 Clarification from Matt Smith (2025-11-03)

> "Many HTML5 games are entirely rendered via canvas, while all of our games exclusively use the DOM for UI elements. Our agent also has a sense of what the input schema is when it builds a game, we can expose this to the QA agent so it has an idea of what controls to test (perhaps as an input prompt)."

### 1.2 DreamUp Game Engine Architecture

DreamUp games are built using a **scene stack system**:

#### Scene Types:
1. **Canvas2D & Canvas3D Scenes**
   - Provide full ECS (Entity Component System) runtimes
   - Include physics, rendering, and game logic
   - Use canvas for graphical content

2. **UI Scenes**
   - Pure DOM elements (divs, buttons, etc.)
   - Handle menus, HUDs, dialogs
   - Interact with players through standard HTML elements

3. **Composite Scenes**
   - Layer multiple child scenes together
   - Common pattern: Canvas game scene + UI overlay for HUD
   - Automatically manage scene lifecycle (mount, unmount, update, draw)

#### Input System:
- **Two-layer architecture:**
  - Low-level: Hardware capture (keys, mouse, pointer)
  - High-level: Gameplay abstractions (Actions, Axes)

- **Actions:** Map multiple inputs to named events
  - Track states: pressed, down, released, hold duration
  - Example: "jump" action can be triggered by spacebar, W key, or touch button

- **Axes:** Provide continuous values
  - 1D axes: Return [-1, 1] (e.g., horizontal movement)
  - 2D axes: Return {x, y} vectors (e.g., joystick)
  - Support WASD, arrow keys, virtual joysticks, D-pads

- **Key Insight:** Game code queries high-level abstractions through `InputManager`, allowing keyboard, touch, and virtual controls to work interchangeably

### 1.3 Implications for QA Agent

| Aspect | External HTML5 Games | DreamUp Games |
|--------|---------------------|---------------|
| **Rendering** | Canvas 2D/WebGL | DOM elements |
| **UI Elements** | Overlaid HTML buttons | Native DOM scenes |
| **Input Target** | Canvas element (needs focus) | Window or specific DOM elements |
| **Event Dispatch** | `canvas.dispatchEvent()` | `window.dispatchEvent()` |
| **Control Schema** | Unknown/guessed | Provided by game dev agent |
| **Detection Strategy** | Look for `<canvas>` | Check for DreamUp DOM patterns |

---

## 2. Updated Functional Requirements

### FR-4.1: Dual-Mode Game Type Detection

**Priority:** P0 (Must Have)

**Description:** The QA agent must automatically detect whether a game is:
- **Canvas-based** (external HTML5 games like Poki/Kongregate)
- **DOM-based** (DreamUp generated games)
- **Hybrid** (canvas + significant DOM UI)

**Acceptance Criteria:**
- ✅ Agent can detect canvas games by presence of `<canvas>` elements
- ✅ Agent can detect DreamUp games by DOM patterns (e.g., `.dreamup-scene`, `#game-container`)
- ✅ Agent logs detected game type for debugging
- ✅ Agent can be overridden with explicit game type parameter
- ✅ Default behavior if ambiguous: Treat as DOM-based

**Detection Algorithm:**
```go
func DetectGameType(ctx context.Context) GameType {
    detector := NewUIDetector(ctx)

    // 1. Check for canvas elements
    hasCanvas := detector.HasGameCanvas()

    // 2. Check for DreamUp-specific DOM patterns
    hasDreamUpScene := detector.DetectElement(".dreamup-scene", UITypeDiv)
    hasGameContainer := detector.DetectElement("#game-container:not(canvas)", UITypeDiv)

    // 3. Check for interactive DOM elements
    interactiveCount := detector.CountInteractiveElements()

    // Decision logic
    if hasDreamUpScene != nil || (hasGameContainer != nil && !hasCanvas) {
        return GameTypeDOM
    }

    if hasCanvas && interactiveCount > 5 {
        return GameTypeHybrid
    }

    if hasCanvas {
        return GameTypeCanvas
    }

    // Default to DOM if uncertain
    return GameTypeDOM
}
```

---

### FR-4.2: Input Schema Handling

**Priority:** P0 (Must Have)

**Description:** The QA agent must accept an input schema from the game dev agent describing:
- Control layout (e.g., "arrow keys to move, spacebar to jump")
- Expected UI elements (buttons, menus)
- Special interaction patterns

**Supported Formats:**

#### Format 1: JSON (Structured)
```json
{
  "game_type": "dom",
  "controls": {
    "move_up": "ArrowUp",
    "move_down": "ArrowDown",
    "move_left": "ArrowLeft",
    "move_right": "ArrowRight",
    "jump": "Space",
    "action": "Enter"
  },
  "axes": {
    "movement": {
      "type": "2d",
      "keys": ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"]
    }
  },
  "ui_elements": [
    {"selector": "#start-button", "type": "button"},
    {"selector": "#pause-button", "type": "button"},
    {"selector": ".score-display", "type": "div"}
  ]
}
```

#### Format 2: Natural Language
```text
Use arrow keys to move the character. Press spacebar to jump.
The game has a start button with class 'btn-start' and a pause
button in the top-right corner. This is a DOM-based game using
the DreamUp engine.
```

**Acceptance Criteria:**
- ✅ Agent can parse JSON schema into structured control mapping
- ✅ Agent can parse natural language schema using LLM
- ✅ Agent validates schema format and provides clear error messages
- ✅ Agent caches parsed schema to avoid re-parsing
- ✅ Agent uses schema to guide gameplay simulation
- ✅ Agent falls back to default controls if schema is invalid/missing

---

### FR-4.3: DOM-Based Game Interaction

**Priority:** P0 (Must Have)

**Description:** For DOM-based games, the QA agent must:
- Send keyboard events to `window` or specific DOM elements (NOT canvas)
- Detect and interact with interactive DOM elements
- Support scene stack transitions
- Handle focus management for DOM controls

**Interaction Modes:**

| Mode | Target | Use Case |
|------|--------|----------|
| **Window** | `window` object | Global keyboard events for DOM games |
| **Element** | Specific DOM element | Focused element interaction |
| **Canvas** | Canvas element | External HTML5 canvas games |

**Acceptance Criteria:**
- ✅ Agent dispatches events to `window` for DOM games
- ✅ Agent can target specific elements (e.g., input fields, buttons)
- ✅ Agent verifies events reach target (checks event listeners)
- ✅ Agent handles scene transitions (detects new scenes loading)
- ✅ Agent respects game type when choosing interaction mode

---

### FR-4.4: Flexible Event Dispatch

**Priority:** P0 (Must Have)

**Description:** The QA agent must dynamically choose event dispatch strategy based on detected game type.

**Dispatch Strategy:**

```go
type InteractionMode string

const (
    ModeCanvas InteractionMode = "canvas"  // Send to canvas element
    ModeDOM    InteractionMode = "dom"     // Send to specific DOM element
    ModeWindow InteractionMode = "window"  // Send to window (global)
)

func (a *Agent) DispatchKeyboardEvent(key string, mode InteractionMode) error {
    switch mode {
    case ModeCanvas:
        return a.SendKeyboardEventToCanvas(key)
    case ModeDOM:
        return a.SendKeyboardEventToElement(key, targetSelector)
    case ModeWindow:
        return a.SendKeyboardEventToWindow(key)
    }
}
```

**Acceptance Criteria:**
- ✅ Agent selects dispatch mode based on game type
- ✅ Canvas mode uses existing `SendKeyboardEventToCanvas()`
- ✅ Window mode dispatches to `window` object
- ✅ DOM mode can target specific elements
- ✅ All modes create proper `KeyboardEvent` objects
- ✅ Events include correct `key`, `code`, `keyCode` properties

---

## 3. Technical Architecture Changes

### 3.1 New Interfaces & Types

#### GameType Enumeration
```go
// internal/agent/game_type.go
type GameType string

const (
    GameTypeCanvas GameType = "canvas"  // External HTML5 canvas games
    GameTypeDOM    GameType = "dom"     // DreamUp DOM-based games
    GameTypeHybrid GameType = "hybrid"  // Canvas + significant DOM UI
)
```

#### Input Schema Parser
```go
// internal/agent/input_schema.go
type InputSchema struct {
    Format      string            // "json" or "natural_language"
    RawSchema   string            // Original schema string
    GameType    GameType          // Specified or detected game type
    Controls    map[string]string // e.g., {"move": "arrow_keys", "jump": "spacebar"}
    Axes        map[string]AxisConfig
    UIElements  []UIElement       // Expected DOM elements
}

type AxisConfig struct {
    Type string   // "1d" or "2d"
    Keys []string // Key bindings
}

func ParseInputSchema(schema string) (*InputSchema, error)
func (s *InputSchema) ValidateControls() error
```

#### Game Detector
```go
// internal/agent/game_detector.go
type GameDetector struct {
    ctx         context.Context
    detector    *UIDetector
    gameType    GameType
    confidence  float64  // 0.0-1.0
}

func NewGameDetector(ctx context.Context) *GameDetector
func (gd *GameDetector) DetectGameType() (GameType, error)
func (gd *GameDetector) CountInteractiveElements() int
func (gd *GameDetector) HasDreamUpPatterns() bool
```

### 3.2 Refactored Code Organization

#### Current Structure (Canvas-focused):
```
internal/agent/
  ├── browser.go          ✅ Reusable
  ├── ui_detection.go     ⚠️  Canvas-specific methods need extraction
  ├── interactions.go     ⚠️  Some canvas assumptions
  ├── evidence.go         ✅ Reusable
  └── evaluator.go        ✅ Reusable
```

#### Proposed Structure (Dual-mode):
```
internal/agent/
  ├── browser.go               ✅ Generic browser lifecycle
  ├── game_type.go             🆕 GameType enum
  ├── game_detector.go         🆕 Game type detection
  ├── input_schema.go          🆕 Input schema parsing
  ├── ui_detection.go          ✅ Generic UI detection (keep)
  ├── canvas_interactions.go   🆕 Canvas-specific (extracted)
  ├── dom_interactions.go      🆕 DOM-specific
  ├── window_interactions.go   🆕 Window-level events
  ├── interactions.go          🔄 Unified dispatcher
  ├── evidence.go              ✅ No changes needed
  └── evaluator.go             ✅ No changes needed
```

### 3.3 Backward Compatibility Strategy

**Principle:** Existing tests with canvas games must continue to work without modification.

**Implementation:**
1. **Default behavior:** Auto-detect game type
2. **Explicit override:** Allow `--game-type=canvas` parameter
3. **Fallback logic:** If detection fails, try canvas mode first (current behavior)
4. **Gradual migration:** Canvas mode remains default for ambiguous cases

---

## 4. Implementation Strategy

### Phase 1: Refactor Canvas-Specific Code (1 day)

**Goal:** Extract canvas logic without breaking existing functionality

**Tasks:**
1. ✅ Create `canvas_interactions.go`
2. ✅ Move `FocusGameCanvas()` to canvas file
3. ✅ Move `SendKeyboardEventToCanvas()` to canvas file
4. ✅ Move `WaitForGameReady()` to canvas file
5. ✅ Update imports in `main.go`
6. ✅ Run existing tests to verify nothing broke

**Success Criteria:** All existing canvas game tests pass

---

### Phase 2: Add Game Type Detection (0.5 days)

**Goal:** Implement auto-detection of canvas vs DOM games

**Tasks:**
1. ✅ Create `game_type.go` with GameType enum
2. ✅ Create `game_detector.go` with detection logic
3. ✅ Implement `DetectGameType()` function
4. ✅ Add `HasDreamUpPatterns()` helper
5. ✅ Add `CountInteractiveElements()` helper
6. ✅ Add logging for detected game type
7. ✅ Test with canvas and DOM games

**Success Criteria:**
- Canvas games detected correctly (Poki, Kongregate)
- DOM games detected correctly (DreamUp examples)

---

### Phase 3: Input Schema Support (1 day)

**Goal:** Parse and use input schema from game dev agent

**Tasks:**
1. ✅ Create `input_schema.go`
2. ✅ Implement JSON schema parser
3. ✅ Implement natural language parser (using LLM)
4. ✅ Add schema validation
5. ✅ Update `main.go` to accept schema parameter
6. ✅ Use schema to guide gameplay simulation
7. ✅ Add fallback for missing/invalid schema

**Success Criteria:**
- JSON schemas parse correctly
- Natural language schemas parse with LLM
- Gameplay uses schema-defined controls

---

### Phase 4: DOM Interaction Support (1 day)

**Goal:** Implement window-level and DOM-targeted event dispatch

**Tasks:**
1. ✅ Create `window_interactions.go`
2. ✅ Implement `SendKeyboardEventToWindow()`
3. ✅ Create `dom_interactions.go`
4. ✅ Implement `SendKeyboardEventToElement()`
5. ✅ Update `interactions.go` with unified dispatcher
6. ✅ Add `DispatchKeyboardEvent()` with mode selection
7. ✅ Update `main.go` to use new dispatcher

**Success Criteria:**
- Window events work for DOM games
- Element-targeted events work
- Mode selection works correctly

---

### Phase 5: Testing & Validation (1 day)

**Goal:** Verify dual-mode works end-to-end

**Tasks:**
1. ✅ Test with external canvas games (Subway Surfers, etc.)
2. ✅ Test with DreamUp DOM games (examples from Matt)
3. ✅ Test with hybrid games (canvas + DOM)
4. ✅ Test input schema parsing (JSON + natural language)
5. ✅ Test explicit game type override
6. ✅ Verify backward compatibility
7. ✅ Update documentation

**Success Criteria:**
- All game types work correctly
- No regressions in existing tests
- Documentation is accurate

---

## 5. Success Criteria

### Functional Success:
- ✅ QA agent correctly detects canvas vs DOM games
- ✅ QA agent parses input schemas (JSON and natural language)
- ✅ Keyboard events reach target (window, canvas, or element)
- ✅ Existing canvas game tests continue to pass
- ✅ DreamUp DOM games can be tested successfully

### Quality Success:
- ✅ Code is well-organized and maintainable
- ✅ Clear separation of canvas vs DOM logic
- ✅ Comprehensive error handling and logging
- ✅ Documentation explains dual-mode architecture

### Performance Success:
- ✅ Game type detection adds <1s overhead
- ✅ Schema parsing adds <500ms overhead
- ✅ No regression in test execution speed

---

## 6. Open Questions & Clarifications Needed

### For Matt Smith:

1. **DreamUp Game Examples**
   - Can you provide 2-3 example DreamUp games for testing?
   - What DOM patterns are most reliable for detection? (`.dreamup-scene`, `data-dreamup-*`, etc.)

2. **Input Schema Format**
   - Is there a preferred format (JSON vs natural language)?
   - Will schema always include game type, or should we detect it?
   - What should we do if schema and detected type conflict?

3. **Scene Stack Transitions**
   - How should we detect scene changes? (DOM mutations, specific events?)
   - Should we wait for scenes to fully mount before interacting?

4. **Event Handling**
   - Do DreamUp games always listen on `window`, or do some use element-specific listeners?
   - Are there any special event properties we need to set?

5. **Testing Scope**
   - Should we test scene transitions (e.g., menu → game → game over)?
   - Should we test UI scenes separately from game scenes?

---

## Appendix A: Comparison Table

| Feature | Canvas Games (External) | DOM Games (DreamUp) |
|---------|------------------------|---------------------|
| **Rendering** | Canvas 2D/WebGL context | DOM elements (divs, buttons, etc.) |
| **Game Logic** | JavaScript in canvas | ECS runtime + DOM scenes |
| **UI Elements** | Overlay HTML buttons | Native DOM scene elements |
| **Input Target** | Canvas element (needs focus) | Window or specific elements |
| **Event Dispatch** | `canvas.dispatchEvent()` | `window.dispatchEvent()` |
| **Focus Management** | Must set `tabindex`, call `focus()` | Usually no explicit focus needed |
| **Detection Method** | Look for `<canvas>` tag | Check for DreamUp patterns |
| **Scene System** | Single canvas, layered rendering | Scene stack (Canvas + UI + Composite) |
| **Input Abstraction** | None (direct keyboard events) | Actions + Axes (InputManager) |
| **Testing Strategy** | Check canvas pixel data | Check DOM state, visibility |
| **Control Schema** | Unknown, must guess | Provided by game dev agent |

---

## Appendix B: Input Schema Examples

### Example 1: Simple Platformer (JSON)
```json
{
  "game_type": "dom",
  "controls": {
    "move_left": "ArrowLeft",
    "move_right": "ArrowRight",
    "jump": "Space"
  },
  "axes": {
    "horizontal": {
      "type": "1d",
      "keys": ["ArrowLeft", "ArrowRight"]
    }
  },
  "ui_elements": [
    {"selector": "#start-btn", "type": "button"},
    {"selector": ".lives-counter", "type": "div"}
  ]
}
```

### Example 2: Twin-Stick Shooter (JSON)
```json
{
  "game_type": "hybrid",
  "controls": {
    "shoot": "Space",
    "reload": "r",
    "pause": "Escape"
  },
  "axes": {
    "movement": {
      "type": "2d",
      "keys": ["w", "a", "s", "d"]
    },
    "aim": {
      "type": "2d",
      "device": "mouse"
    }
  },
  "ui_elements": [
    {"selector": "#ammo-display", "type": "div"},
    {"selector": "#health-bar", "type": "progress"}
  ]
}
```

### Example 3: Natural Language (Simple)
```text
Use WASD keys to move the player. Press spacebar to jump.
The game has a start button and a score display in the top-left.
```

### Example 4: Natural Language (Detailed)
```text
This is a DOM-based puzzle game built with DreamUp. Players use
the mouse to click and drag tiles. The game has three UI scenes:
- Main menu (button with class "btn-play")
- Game board (grid of draggable divs)
- Victory screen (appears when puzzle solved)

No keyboard controls are used. All interaction is mouse-based.
```

---

## Appendix C: Detection Algorithm Pseudocode

```python
def detect_game_type(page):
    # Step 1: Look for explicit signals
    if has_meta_tag(page, "dreamup:game-type"):
        return meta_tag_value

    # Step 2: Check for DreamUp-specific patterns
    if has_element(page, ".dreamup-scene") or \
       has_element(page, "[data-dreamup-engine]"):
        return "dom"

    # Step 3: Check for canvas
    canvas_count = count_elements(page, "canvas")
    interactive_dom_count = count_interactive_elements(page)

    if canvas_count == 0:
        return "dom"

    if canvas_count > 0 and interactive_dom_count > 5:
        return "hybrid"

    if canvas_count > 0:
        return "canvas"

    # Step 4: Default fallback
    return "dom"  # Assume DreamUp game if uncertain
```

---

## Revision History

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | 2025-11-03 | Initial PRD update document | Reuben |

---

**End of Document**
