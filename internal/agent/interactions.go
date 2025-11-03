package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// ActionType represents the type of interaction action
type ActionType string

const (
	// ActionClick performs a mouse click on an element
	ActionClick ActionType = "click"
	// ActionKeypress simulates a keyboard key press
	ActionKeypress ActionType = "keypress"
	// ActionWait pauses execution for a specified duration
	ActionWait ActionType = "wait"
	// ActionScreenshot captures a screenshot at this point
	ActionScreenshot ActionType = "screenshot"
)

// Action represents a single interaction action to perform
type Action struct {
	// Type is the kind of action to execute
	Type ActionType
	// Selector is the CSS selector for click actions
	Selector string
	// Key is the keyboard key for keypress actions (e.g., "ArrowUp", "Space", "Enter")
	Key string
	// Duration is the wait time for wait actions
	Duration time.Duration
	// Context is the screenshot context for screenshot actions
	Context ScreenshotContext
	// Timeout is the maximum time to wait for this action to complete
	Timeout time.Duration
	// Description is a human-readable description of this action
	Description string
}

// InteractionPlan represents a sequence of actions to execute
type InteractionPlan struct {
	// Name is a descriptive name for this interaction plan
	Name string
	// Actions is the ordered list of actions to execute
	Actions []Action
	// DefaultTimeout is the default timeout for actions that don't specify one
	DefaultTimeout time.Duration
}

// NewClickAction creates a new click action
func NewClickAction(selector, description string) Action {
	return Action{
		Type:        ActionClick,
		Selector:    selector,
		Description: description,
		Timeout:     30 * time.Second,
	}
}

// NewKeypressAction creates a new keypress action
func NewKeypressAction(key, description string) Action {
	return Action{
		Type:        ActionKeypress,
		Key:         key,
		Description: description,
		Timeout:     5 * time.Second,
	}
}

// NewWaitAction creates a new wait action
func NewWaitAction(duration time.Duration, description string) Action {
	return Action{
		Type:        ActionWait,
		Duration:    duration,
		Description: description,
	}
}

// NewScreenshotAction creates a new screenshot action
func NewScreenshotAction(context ScreenshotContext, description string) Action {
	return Action{
		Type:        ActionScreenshot,
		Context:     context,
		Description: description,
		Timeout:     10 * time.Second,
	}
}

// NewStandardGamePlan creates a standard interaction plan for game testing
func NewStandardGamePlan() InteractionPlan {
	return InteractionPlan{
		Name:           "Standard Game Test",
		DefaultTimeout: 30 * time.Second,
		Actions: []Action{
			NewScreenshotAction(ContextInitial, "Capture initial game state"),
			NewWaitAction(2*time.Second, "Wait for game to fully load"),
			NewClickAction("button.start, #start-button, .play-button", "Click start button"),
			NewWaitAction(1*time.Second, "Wait after clicking start"),
			NewScreenshotAction(ContextGameplay, "Capture gameplay started"),
			NewKeypressAction("ArrowUp", "Press up arrow"),
			NewWaitAction(500*time.Millisecond, "Short wait"),
			NewKeypressAction("Space", "Press spacebar"),
			NewWaitAction(500*time.Millisecond, "Short wait"),
			NewKeypressAction("ArrowDown", "Press down arrow"),
			NewWaitAction(2*time.Second, "Wait for gameplay actions"),
			NewScreenshotAction(ContextFinal, "Capture final game state"),
		},
	}
}

// ExecuteAction executes a single action using chromedp
func ExecuteAction(ctx context.Context, action Action) (*Screenshot, error) {
	switch action.Type {
	case ActionClick:
		return nil, executeClick(ctx, action)
	case ActionKeypress:
		return nil, executeKeypress(ctx, action)
	case ActionWait:
		return nil, executeWait(action)
	case ActionScreenshot:
		return executeScreenshot(ctx, action)
	default:
		return nil, fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeScreenshot captures a screenshot
func executeScreenshot(ctx context.Context, action Action) (*Screenshot, error) {
	screenshot, err := CaptureScreenshot(ctx, action.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot for %s: %w", action.Context, err)
	}

	if err := screenshot.SaveToTemp(); err != nil {
		return nil, fmt.Errorf("failed to save screenshot: %w", err)
	}

	return screenshot, nil
}

// executeClick performs a click action on a specified selector
func executeClick(ctx context.Context, action Action) error {
	timeout := action.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := chromedp.Run(timeoutCtx,
		chromedp.WaitVisible(action.Selector, chromedp.ByQuery),
		chromedp.Click(action.Selector, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to click %s: %w", action.Selector, err)
	}

	return nil
}

// executeKeypress simulates a keyboard key press
func executeKeypress(ctx context.Context, action Action) error {
	timeout := action.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Map common key names to key strings and use SendKeys
	var keyToSend string
	switch action.Key {
	case "ArrowUp":
		keyToSend = "\ue013" // Unicode for ArrowUp
	case "ArrowDown":
		keyToSend = "\ue015" // Unicode for ArrowDown
	case "ArrowLeft":
		keyToSend = "\ue012" // Unicode for ArrowLeft
	case "ArrowRight":
		keyToSend = "\ue014" // Unicode for ArrowRight
	case "Space", " ":
		keyToSend = " "
	case "Enter":
		keyToSend = "\r"
	case "Escape":
		keyToSend = "\ue00c" // Unicode for Escape
	default:
		// For single character keys, use as-is
		if len(action.Key) == 1 {
			keyToSend = action.Key
		} else {
			return fmt.Errorf("unknown key: %s", action.Key)
		}
	}

	err := chromedp.Run(timeoutCtx,
		chromedp.SendKeys("body", keyToSend, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to press key %s: %w", action.Key, err)
	}

	return nil
}

// executeWait pauses execution for the specified duration
func executeWait(action Action) error {
	time.Sleep(action.Duration)
	return nil
}

// ExecutePlan executes an entire interaction plan
func ExecutePlan(ctx context.Context, plan InteractionPlan) ([]*Screenshot, error) {
	screenshots := make([]*Screenshot, 0)

	for i, action := range plan.Actions {
		screenshot, err := ExecuteAction(ctx, action)
		if err != nil {
			return screenshots, fmt.Errorf("failed to execute action %d (%s): %w", i, action.Description, err)
		}

		if screenshot != nil {
			screenshots = append(screenshots, screenshot)
		}
	}

	return screenshots, nil
}
