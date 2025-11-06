package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	openai "github.com/sashabaranov/go-openai"
)

// GameplayAgent coordinates vision-based gameplay with mouse actions
// Inspired by Stagehand's AI-powered action sequencing and self-healing patterns
type GameplayAgent struct {
	ctx          context.Context
	vision       *VisionDOMDetector
	client       *openai.Client
	actionCache  *ActionCache
	gridCols     int // 20 columns (A-T)
	gridRows     int // 12 rows (1-12)
	imageWidth   int // 1280
	imageHeight  int // 720
}

// GameplayActionType represents different types of gameplay actions
type GameplayActionType string

const (
	ActionTypeDetectElement  GameplayActionType = "detect_element"  // Find an element (slingshot, target, etc.)
	ActionTypeDragSlingshot  GameplayActionType = "drag_slingshot"  // Drag slingshot to aim
	ActionTypeWait           GameplayActionType = "wait"            // Wait for game state to change
	ActionTypeObserve        GameplayActionType = "observe"         // Take screenshot and analyze
	ActionTypeClick          GameplayActionType = "click"           // Single click action
	ActionTypeKeyPress       GameplayActionType = "keypress"        // Single key press (press and release)
	ActionTypeKeyHold        GameplayActionType = "key_hold"        // Press and hold key down
	ActionTypeKeyRelease     GameplayActionType = "key_release"     // Release a held key
	ActionTypeKeySequence    GameplayActionType = "key_sequence"    // Sequence of key presses
)

// GameplayActionPlan represents a single action in a gameplay sequence
type GameplayActionPlan struct {
	Type        GameplayActionType `json:"type"`
	StartCell   string             `json:"start_cell,omitempty"`   // Grid cell to start (e.g., "E7")
	EndCell     string             `json:"end_cell,omitempty"`     // Grid cell to end (e.g., "C5")
	TargetCell  string             `json:"target_cell,omitempty"`  // Grid cell to click (e.g., "J10")
	WaitMs      int                `json:"wait_ms,omitempty"`      // Duration to wait
	Description string             `json:"description"`            // AI reasoning
	ElementName string             `json:"element_name,omitempty"` // What element to find
	Key         string             `json:"key,omitempty"`          // Key for keyboard actions (e.g., "ArrowUp", "w", "Space")
	Keys        []string           `json:"keys,omitempty"`         // Keys for key_sequence actions
	HoldMs      int                `json:"hold_ms,omitempty"`      // Duration to hold key (for key_hold)
}

// SlingshotDragAction represents a slingshot drag with grid-based coordinates
type SlingshotDragAction struct {
	SlingshotCell GridCell // Where the slingshot/bird is located
	TargetCell    GridCell // Where to drag to (aim point)
	AngleDegrees  float64  // Calculated angle
	Power         float64  // Power (0.0-1.0) based on drag distance
	Description   string   // AI reasoning for this shot
}

// ActionCache stores successful gameplay actions for self-healing
// Inspired by Stagehand's caching and self-healing patterns
type ActionCache struct {
	SuccessfulDrags []CachedDrag `json:"successful_drags"`
}

// CachedDrag represents a successful slingshot drag
type CachedDrag struct {
	GameName      string    `json:"game_name"`
	StartCell     string    `json:"start_cell"`
	EndCell       string    `json:"end_cell"`
	Outcome       string    `json:"outcome"`       // "destroyed_pig", "hit_structure", "missed"
	Timestamp     time.Time `json:"timestamp"`
	ScreenshotB64 string    `json:"screenshot_b64"` // Optional: before state
}

// NewGameplayAgent creates a new gameplay agent
func NewGameplayAgent(ctx context.Context, vision *VisionDOMDetector) (*GameplayAgent, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY required for gameplay agent")
	}

	return &GameplayAgent{
		ctx:         ctx,
		vision:      vision,
		client:      openai.NewClient(apiKey),
		actionCache: &ActionCache{SuccessfulDrags: []CachedDrag{}},
		gridCols:    20,
		gridRows:    12,
		imageWidth:  1280,
		imageHeight: 720,
	}, nil
}

// DetectSlingshotAndTarget uses vision to find slingshot and determine optimal aim
func (g *GameplayAgent) DetectSlingshotAndTarget(screenshot *Screenshot, gameMechanics string) (*SlingshotDragAction, error) {
	// Apply grid overlay to screenshot
	griddedScreenshot, err := AddGridOverlay(screenshot, g.gridCols, g.gridRows)
	if err != nil {
		log.Printf("[Gameplay] Warning: Failed to add grid overlay: %v", err)
		griddedScreenshot = screenshot
	}

	imageBase64 := base64.StdEncoding.EncodeToString(griddedScreenshot.Data)

	// Build game mechanics context
	mechanicsContext := ""
	if gameMechanics != "" {
		mechanicsContext = fmt.Sprintf("\n\nGAME MECHANICS:\n%s", gameMechanics)
	}

	prompt := fmt.Sprintf(`Analyze this Angry Birds gameplay screenshot. Grid: %dx%d (columns A-%s, rows 1-%d).
%s

TASK: Identify the slingshot (bird ready to launch) and suggest where to aim.

Return JSON:
{
  "slingshot_cell": "E7",
  "target_aim_cell": "C5",
  "reasoning": "Pull slingshot back and down to hit the bottom wood block",
  "estimated_angle": 45,
  "estimated_power": 0.7
}

GUIDELINES:
- slingshot_cell: Grid cell where the bird/slingshot is currently positioned (usually left side, columns A-F)
- target_aim_cell: Where to drag TO (pull back direction, usually left and/or down from slingshot)
- Power: 0.5 = medium, 0.7 = strong, 1.0 = maximum
- Angle: degrees from horizontal (0 = straight right, 45 = diagonal up-right, etc.)

EXAMPLE:
If slingshot is at E7, you might drag to C5 (back and down) for a low trajectory shot.`,
		g.gridCols, g.gridRows, string(rune('A'+g.gridCols-1)), g.gridRows, mechanicsContext)

	log.Printf("[Gameplay] Sending slingshot detection request to GPT-4o...")
	log.Printf("[Gameplay] Prompt: %s", prompt)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: prompt,
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: fmt.Sprintf("data:image/png;base64,%s", imageBase64),
						},
					},
				},
			},
		},
		MaxCompletionTokens: 800,
	})

	if err != nil {
		return nil, fmt.Errorf("slingshot detection API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from vision API")
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)
	log.Printf("[Gameplay] Response: %s", responseText)

	// Parse JSON response
	var result struct {
		SlingshotCell   string  `json:"slingshot_cell"`
		TargetAimCell   string  `json:"target_aim_cell"`
		Reasoning       string  `json:"reasoning"`
		EstimatedAngle  float64 `json:"estimated_angle"`
		EstimatedPower  float64 `json:"estimated_power"`
	}

	// Extract JSON from markdown code fences if present
	jsonText := responseText
	if strings.Contains(responseText, "```json") {
		start := strings.Index(responseText, "```json")
		end := strings.Index(responseText[start+7:], "```")
		if end != -1 {
			jsonText = responseText[start+7 : start+7+end]
		}
	} else if strings.Contains(responseText, "{") {
		start := strings.Index(responseText, "{")
		end := strings.LastIndex(responseText, "}")
		if start != -1 && end != -1 {
			jsonText = responseText[start : end+1]
		}
	}

	jsonText = strings.TrimSpace(jsonText)

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse slingshot detection response: %w (response: %s)", err, jsonText)
	}

	// Parse grid cells
	slingshotCell, err := parseGridCell(result.SlingshotCell)
	if err != nil {
		return nil, fmt.Errorf("invalid slingshot cell '%s': %w", result.SlingshotCell, err)
	}

	targetCell, err := parseGridCell(result.TargetAimCell)
	if err != nil {
		return nil, fmt.Errorf("invalid target aim cell '%s': %w", result.TargetAimCell, err)
	}

	log.Printf("[Gameplay] Slingshot detected: %s → %s (angle: %.1f°, power: %.2f)",
		slingshotCell.String(), targetCell.String(), result.EstimatedAngle, result.EstimatedPower)
	log.Printf("[Gameplay] Reasoning: %s", result.Reasoning)

	return &SlingshotDragAction{
		SlingshotCell: slingshotCell,
		TargetCell:    targetCell,
		AngleDegrees:  result.EstimatedAngle,
		Power:         result.EstimatedPower,
		Description:   result.Reasoning,
	}, nil
}

// ExecuteDragAction performs the slingshot drag using existing CDP mouse actions
func (g *GameplayAgent) ExecuteDragAction(dragAction *SlingshotDragAction) error {
	// Convert grid cells to pixel coordinates
	startX, startY := dragAction.SlingshotCell.ToPixelCoordinates(
		g.gridCols, g.gridRows, g.imageWidth, g.imageHeight)
	endX, endY := dragAction.TargetCell.ToPixelCoordinates(
		g.gridCols, g.gridRows, g.imageWidth, g.imageHeight)

	log.Printf("[Gameplay] Executing drag from %s (%d,%d) to %s (%d,%d)",
		dragAction.SlingshotCell.String(), startX, startY,
		dragAction.TargetCell.String(), endX, endY)
	log.Printf("[Gameplay] Action: %s", dragAction.Description)

	// Calculate drag duration based on power (more power = slower drag for better control)
	baseDuration := 300 * time.Millisecond
	powerMultiplier := 1.0 + dragAction.Power*0.5 // 1.0-1.5x duration
	dragDuration := time.Duration(float64(baseDuration) * powerMultiplier)

	// Hold duration - longer hold for more power
	holdDuration := time.Duration(100 + int(dragAction.Power*100)) * time.Millisecond

	// Use existing PerformDrag implementation (smooth 10-step CDP drag)
	err := PerformDrag(g.ctx, startX, startY, endX, endY, dragDuration, holdDuration)
	if err != nil {
		return fmt.Errorf("drag execution failed: %w", err)
	}

	log.Printf("[Gameplay] Drag completed successfully (duration: %v, hold: %v)", dragDuration, holdDuration)

	return nil
}

// ExecuteKeyboardAction performs keyboard actions using existing keypress infrastructure
func (g *GameplayAgent) ExecuteKeyboardAction(action *GameplayActionPlan) error {
	switch action.Type {
	case ActionTypeKeyPress:
		return g.executeKeyPress(action.Key)
	case ActionTypeKeyHold:
		return g.executeKeyHold(action.Key, action.HoldMs)
	case ActionTypeKeyRelease:
		return g.executeKeyRelease(action.Key)
	case ActionTypeKeySequence:
		return g.executeKeySequence(action.Keys)
	default:
		return fmt.Errorf("unknown keyboard action type: %s", action.Type)
	}
}

// executeKeyPress simulates a single key press (press and release)
func (g *GameplayAgent) executeKeyPress(key string) error {
	log.Printf("[Gameplay] Pressing key: %s", key)

	// Use existing executeKeypress function via Action struct
	action := NewKeypressAction(key, fmt.Sprintf("Gameplay key press: %s", key))
	err := executeKeypress(g.ctx, action)
	if err != nil {
		return fmt.Errorf("failed to press key %s: %w", key, err)
	}

	log.Printf("[Gameplay] Key press completed: %s", key)
	return nil
}

// executeKeyHold presses and holds a key for a specified duration
func (g *GameplayAgent) executeKeyHold(key string, holdMs int) error {
	log.Printf("[Gameplay] Holding key %s for %dms", key, holdMs)

	// Map key to Unicode for chromedp
	keyCode, err := mapKeyToUnicode(key)
	if err != nil {
		return err
	}

	// Use chromedp to send key down event
	err = chromedp.Run(g.ctx,
		chromedp.KeyEvent(keyCode),
	)
	if err != nil {
		return fmt.Errorf("failed to press down key %s: %w", key, err)
	}

	// Hold for specified duration
	if holdMs > 0 {
		time.Sleep(time.Duration(holdMs) * time.Millisecond)
	}

	log.Printf("[Gameplay] Key hold completed: %s (%dms)", key, holdMs)
	return nil
}

// executeKeyRelease releases a held key
func (g *GameplayAgent) executeKeyRelease(key string) error {
	log.Printf("[Gameplay] Releasing key: %s", key)

	// Map key to Unicode for chromedp
	keyCode, err := mapKeyToUnicode(key)
	if err != nil {
		return err
	}

	// Use chromedp to send key up event
	err = chromedp.Run(g.ctx,
		chromedp.KeyEvent(keyCode),
	)
	if err != nil {
		return fmt.Errorf("failed to release key %s: %w", key, err)
	}

	log.Printf("[Gameplay] Key release completed: %s", key)
	return nil
}

// executeKeySequence executes a sequence of key presses
func (g *GameplayAgent) executeKeySequence(keys []string) error {
	log.Printf("[Gameplay] Executing key sequence: %v", keys)

	for i, key := range keys {
		log.Printf("[Gameplay] Key sequence %d/%d: %s", i+1, len(keys), key)

		err := g.executeKeyPress(key)
		if err != nil {
			return fmt.Errorf("failed to press key %s in sequence: %w", key, err)
		}

		// Small delay between keys in sequence
		time.Sleep(50 * time.Millisecond)
	}

	log.Printf("[Gameplay] Key sequence completed (%d keys)", len(keys))
	return nil
}

// mapKeyToUnicode maps key names to Unicode values for chromedp
func mapKeyToUnicode(key string) (string, error) {
	switch key {
	case "ArrowUp", "Up":
		return "\ue013", nil
	case "ArrowDown", "Down":
		return "\ue015", nil
	case "ArrowLeft", "Left":
		return "\ue012", nil
	case "ArrowRight", "Right":
		return "\ue014", nil
	case "Space", " ":
		return " ", nil
	case "Enter", "Return":
		return "\r", nil
	case "Escape", "Esc":
		return "\ue00c", nil
	case "Tab":
		return "\t", nil
	case "Backspace":
		return "\ue003", nil
	case "Delete":
		return "\ue017", nil
	default:
		// For single character keys (w, a, s, d, etc.), use as-is
		if len(key) == 1 {
			return key, nil
		}
		return "", fmt.Errorf("unknown key: %s", key)
	}
}

// ExecuteGameplayAction executes a single gameplay action from an action plan
// This is the unified execution function that handles all action types
func (g *GameplayAgent) ExecuteGameplayAction(action *GameplayActionPlan) error {
	log.Printf("[Gameplay] Executing action: %s - %s", action.Type, action.Description)

	switch action.Type {
	case ActionTypeDetectElement:
		// Detection actions don't execute anything, they're used for planning
		log.Printf("[Gameplay] Detection action (no execution): looking for %s", action.ElementName)
		return nil

	case ActionTypeDragSlingshot:
		// Parse grid cells and execute drag
		startCell, err := parseGridCell(action.StartCell)
		if err != nil {
			return fmt.Errorf("invalid start cell %s: %w", action.StartCell, err)
		}
		endCell, err := parseGridCell(action.EndCell)
		if err != nil {
			return fmt.Errorf("invalid end cell %s: %w", action.EndCell, err)
		}

		dragAction := &SlingshotDragAction{
			SlingshotCell: startCell,
			TargetCell:    endCell,
			AngleDegrees:  0, // Will be calculated
			Power:         0.7, // Default power
			Description:   action.Description,
		}
		return g.ExecuteDragAction(dragAction)

	case ActionTypeClick:
		// Parse target cell and execute click
		targetCell, err := parseGridCell(action.TargetCell)
		if err != nil {
			return fmt.Errorf("invalid target cell %s: %w", action.TargetCell, err)
		}

		x, y := targetCell.ToPixelCoordinates(g.gridCols, g.gridRows, g.imageWidth, g.imageHeight)
		log.Printf("[Gameplay] Clicking at %s (%d, %d)", action.TargetCell, x, y)

		err = chromedp.Run(g.ctx,
			chromedp.MouseClickXY(float64(x), float64(y)),
		)
		if err != nil {
			return fmt.Errorf("failed to click at %s: %w", action.TargetCell, err)
		}
		return nil

	case ActionTypeKeyPress, ActionTypeKeyHold, ActionTypeKeyRelease, ActionTypeKeySequence:
		// Execute keyboard actions
		return g.ExecuteKeyboardAction(action)

	case ActionTypeWait:
		// Wait for specified duration
		duration := time.Duration(action.WaitMs) * time.Millisecond
		log.Printf("[Gameplay] Waiting for %v", duration)
		time.Sleep(duration)
		return nil

	case ActionTypeObserve:
		// Take screenshot and optionally analyze
		screenshot, err := CaptureScreenshot(g.ctx, ContextGameplay)
		if err != nil {
			return fmt.Errorf("failed to capture screenshot: %w", err)
		}

		// Save for debugging
		timestamp := time.Now().Format("20060102_150405")
		path := fmt.Sprintf("/tmp/gameplay_observe_%s.png", timestamp)
		if err := os.WriteFile(path, screenshot.Data, 0644); err != nil {
			log.Printf("[Gameplay] Warning: Failed to save observation screenshot: %v", err)
		} else {
			log.Printf("[Gameplay] Observation screenshot saved: %s", path)
		}
		return nil

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// ExecuteGameplaySequence executes a full sequence of gameplay actions
func (g *GameplayAgent) ExecuteGameplaySequence(actions []GameplayActionPlan) error {
	log.Printf("[Gameplay] Executing action sequence (%d actions)", len(actions))

	for i, action := range actions {
		log.Printf("[Gameplay] === Action %d/%d: %s ===", i+1, len(actions), action.Type)

		err := g.ExecuteGameplayAction(&action)
		if err != nil {
			return fmt.Errorf("action %d (%s) failed: %w", i+1, action.Type, err)
		}

		log.Printf("[Gameplay] Action %d completed: %s", i+1, action.Description)

		// Small delay between actions for stability
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("[Gameplay] Action sequence completed successfully")
	return nil
}

// PlayGameLevel executes a full gameplay loop for one level attempt
func (g *GameplayAgent) PlayGameLevel(gameName string, gameMechanics string, maxAttempts int) error {
	log.Printf("[Gameplay] Starting gameplay loop for %s (max attempts: %d)", gameName, maxAttempts)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("[Gameplay] === Attempt %d/%d ===", attempt, maxAttempts)

		// 1. Capture current game state
		screenshot, err := CaptureScreenshot(g.ctx, ContextGameplay)
		if err != nil {
			return fmt.Errorf("failed to capture screenshot: %w", err)
		}

		// Save screenshot for debugging
		timestamp := time.Now().Format("20060102_150405")
		screenshotPath := fmt.Sprintf("/tmp/gameplay_%s_attempt%d.png", timestamp, attempt)
		if err := os.WriteFile(screenshotPath, screenshot.Data, 0644); err != nil {
			log.Printf("[Gameplay] Warning: Failed to save screenshot: %v", err)
		} else {
			log.Printf("[Gameplay] Screenshot saved: %s", screenshotPath)
		}

		// 2. Detect slingshot and calculate optimal aim
		dragAction, err := g.DetectSlingshotAndTarget(screenshot, gameMechanics)
		if err != nil {
			log.Printf("[Gameplay] Failed to detect slingshot: %v", err)
			// Wait and try again
			time.Sleep(2 * time.Second)
			continue
		}

		// 3. Execute the drag action
		if err := g.ExecuteDragAction(dragAction); err != nil {
			log.Printf("[Gameplay] Failed to execute drag: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// 4. Wait for game physics to settle
		log.Printf("[Gameplay] Waiting for game physics to complete...")
		time.Sleep(5 * time.Second)

		// 5. Capture result screenshot to analyze outcome
		resultScreenshot, err := CaptureScreenshot(g.ctx, ContextGameplay)
		if err != nil {
			log.Printf("[Gameplay] Warning: Failed to capture result screenshot: %v", err)
		} else {
			resultPath := fmt.Sprintf("/tmp/gameplay_%s_attempt%d_result.png", timestamp, attempt)
			if err := os.WriteFile(resultPath, resultScreenshot.Data, 0644); err != nil {
				log.Printf("[Gameplay] Warning: Failed to save result screenshot: %v", err)
			} else {
				log.Printf("[Gameplay] Result screenshot saved: %s", resultPath)
			}

			// Analyze outcome (simple version - could be enhanced with vision API)
			outcome := g.analyzeOutcome(resultScreenshot)
			log.Printf("[Gameplay] Outcome: %s", outcome)

			// Cache successful actions for self-healing
			if outcome == "success" || strings.Contains(outcome, "destroyed") {
				g.CacheSuccessfulDrag(gameName, dragAction, outcome, screenshot)
			}
		}

		// 6. Check if level is complete (could use vision API to detect win/lose screens)
		log.Printf("[Gameplay] Checking if level is complete...")
		// For now, just continue to next attempt
		time.Sleep(2 * time.Second)
	}

	log.Printf("[Gameplay] Completed gameplay loop (%d attempts)", maxAttempts)
	return nil
}

// analyzeOutcome performs basic outcome analysis
// Could be enhanced with vision API to detect specific results
func (g *GameplayAgent) analyzeOutcome(screenshot *Screenshot) string {
	// Simple placeholder - in a full implementation, this would use vision API
	// to analyze if pigs were destroyed, structures collapsed, etc.
	return "unknown"
}

// CacheSuccessfulDrag stores a successful drag action for future reference
// Implements Stagehand's self-healing pattern
func (g *GameplayAgent) CacheSuccessfulDrag(gameName string, action *SlingshotDragAction, outcome string, screenshot *Screenshot) {
	// Encode screenshot to base64 (optional - could skip to save memory)
	screenshotB64 := ""
	if screenshot != nil && len(screenshot.Data) < 500*1024 { // Only cache if < 500KB
		screenshotB64 = base64.StdEncoding.EncodeToString(screenshot.Data)
	}

	cached := CachedDrag{
		GameName:      gameName,
		StartCell:     action.SlingshotCell.String(),
		EndCell:       action.TargetCell.String(),
		Outcome:       outcome,
		Timestamp:     time.Now(),
		ScreenshotB64: screenshotB64,
	}

	g.actionCache.SuccessfulDrags = append(g.actionCache.SuccessfulDrags, cached)
	log.Printf("[Gameplay Cache] Cached successful drag: %s → %s (outcome: %s)",
		cached.StartCell, cached.EndCell, outcome)

	// Limit cache size to last 50 successful drags
	if len(g.actionCache.SuccessfulDrags) > 50 {
		g.actionCache.SuccessfulDrags = g.actionCache.SuccessfulDrags[len(g.actionCache.SuccessfulDrags)-50:]
	}
}

// GetCachedDragsForGame returns cached successful drags for a specific game
func (g *GameplayAgent) GetCachedDragsForGame(gameName string) []CachedDrag {
	var drags []CachedDrag
	for _, drag := range g.actionCache.SuccessfulDrags {
		if drag.GameName == gameName {
			drags = append(drags, drag)
		}
	}
	return drags
}

// PlanGameplaySequence generates a sequence of actions using AI
// Implements Stagehand's action sequencing pattern
func (g *GameplayAgent) PlanGameplaySequence(screenshot *Screenshot, gameMechanics string) ([]GameplayActionPlan, error) {
	griddedScreenshot, err := AddGridOverlay(screenshot, g.gridCols, g.gridRows)
	if err != nil {
		griddedScreenshot = screenshot
	}

	imageBase64 := base64.StdEncoding.EncodeToString(griddedScreenshot.Data)

	mechanicsContext := ""
	if gameMechanics != "" {
		mechanicsContext = fmt.Sprintf("\n\nGAME MECHANICS:\n%s", gameMechanics)
	}

	prompt := fmt.Sprintf(`Plan a sequence of actions to play this game. Grid: %dx%d (A-%s, 1-%d).
%s

Return JSON array of actions:
[
  {"type": "detect_element", "element_name": "slingshot", "description": "find bird position"},
  {"type": "drag_slingshot", "start_cell": "E7", "end_cell": "C5", "description": "aim at bottom structure"},
  {"type": "keypress", "key": "ArrowUp", "description": "move character up"},
  {"type": "key_sequence", "keys": ["w", "w", "d"], "description": "move forward twice and turn right"},
  {"type": "wait", "wait_ms": 5000, "description": "wait for physics"},
  {"type": "observe", "description": "check game state"}
]

Available action types:
- detect_element: Find a game element (element_name)
- drag_slingshot: Drag from start_cell to end_cell (for slingshot games)
- click: Single click at target_cell (for button presses, menu items)
- keypress: Single key press and release (key: "ArrowUp", "w", "Space", etc.)
- key_hold: Press and hold key (key, hold_ms)
- key_release: Release a held key (key)
- key_sequence: Sequence of key presses (keys: ["w", "a", "s", "d"])
- wait: Wait for wait_ms milliseconds (for game state changes, animations)
- observe: Analyze current game state (take screenshot and analyze)

KEYBOARD KEYS:
- Arrow keys: "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight" (or "Up", "Down", "Left", "Right")
- Letter keys: "w", "a", "s", "d", etc. (lowercase single characters)
- Special keys: "Space", "Enter", "Escape", "Tab", "Backspace", "Delete"

GUIDELINES:
- For keyboard-based games (Pac-Man, platformers, etc.), use keypress or key_sequence
- For mouse-based games (Angry Birds, etc.), use drag_slingshot or click
- Use observe to check game state before and after actions
- Use wait to let animations/physics complete
- Choose actions based on what you see in the game screenshot`,
		g.gridCols, g.gridRows, string(rune('A'+g.gridCols-1)), g.gridRows, mechanicsContext)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: prompt,
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: fmt.Sprintf("data:image/png;base64,%s", imageBase64),
						},
					},
				},
			},
		},
		MaxCompletionTokens: 1000,
	})

	if err != nil {
		return nil, fmt.Errorf("action planning API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from vision API")
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Parse JSON array
	jsonText := responseText
	if strings.Contains(responseText, "```json") {
		start := strings.Index(responseText, "```json")
		end := strings.Index(responseText[start+7:], "```")
		if end != -1 {
			jsonText = responseText[start+7 : start+7+end]
		}
	} else if strings.Contains(responseText, "[") {
		start := strings.Index(responseText, "[")
		end := strings.LastIndex(responseText, "]")
		if start != -1 && end != -1 {
			jsonText = responseText[start : end+1]
		}
	}

	var actions []GameplayActionPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonText)), &actions); err != nil {
		return nil, fmt.Errorf("failed to parse action sequence: %w (response: %s)", err, jsonText)
	}

	log.Printf("[Gameplay] Planned %d actions", len(actions))
	for i, action := range actions {
		log.Printf("[Gameplay]   %d. %s: %s", i+1, action.Type, action.Description)
	}

	return actions, nil
}
