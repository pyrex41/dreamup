package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/chromedp/chromedp"
	openai "github.com/sashabaranov/go-openai"
)

// VisionDOMDetector uses GPT-4o vision to identify elements by description, then finds them via DOM
type VisionDOMDetector struct {
	ctx    context.Context
	client *openai.Client
}

// NewVisionDOMDetector creates a new vision-based DOM detector
func NewVisionDOMDetector(ctx context.Context) (*VisionDOMDetector, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	client := openai.NewClient(apiKey)

	return &VisionDOMDetector{
		ctx:    ctx,
		client: client,
	}, nil
}

// DetectStartButtonDescription uses vision to describe what the start button looks like
func (v *VisionDOMDetector) DetectStartButtonDescription(screenshot *Screenshot) (string, error) {
	// Encode screenshot to base64
	imageBase64 := base64.StdEncoding.EncodeToString(screenshot.Data)

	// Create vision request
	resp, err := v.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{
							Type: openai.ChatMessagePartTypeText,
							Text: `Analyze this game screenshot and find the start/play button.

Describe the button's TEXT CONTENT only. Return ONLY a JSON object:
{
  "found": true/false,
  "text": "exact text on the button (e.g., START GAME, PLAY, BEGIN)",
  "confidence": 0.0-1.0
}

Look for buttons with text like:
- START GAME
- PLAY
- START
- PLAY NOW
- BEGIN
- GO

Return the EXACT text you see on the button, in the same case.`,
						},
						{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL:    fmt.Sprintf("data:image/png;base64,%s", imageBase64),
								Detail: openai.ImageURLDetailAuto,
							},
						},
					},
				},
			},
			MaxTokens: 150,
		},
	)

	if err != nil {
		return "", fmt.Errorf("vision API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from vision API")
	}

	// Parse JSON response
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result struct {
		Found      bool    `json:"found"`
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", fmt.Errorf("failed to parse vision response: %w (content: %s)", err, content)
	}

	if !result.Found {
		return "", fmt.Errorf("no start button detected")
	}

	return result.Text, nil
}

// ClickButtonByText finds and clicks a button by its text content using DOM queries
func (v *VisionDOMDetector) ClickButtonByText(buttonText string) error {
	script := fmt.Sprintf(`
(function() {
	const searchText = %q;
	console.log('[ClickByText] Searching for button with text:', searchText);

	// Find ALL elements with matching text first (very broad search)
	const allElements = document.querySelectorAll('*');
	const candidates = [];

	for (let elem of allElements) {
		const text = elem.textContent?.trim() || '';
		const textUpper = text.toUpperCase();
		const searchUpper = searchText.toUpperCase();

		// Only include elements that directly contain the text (not inherited from children)
		if (textUpper === searchUpper || (textUpper.includes(searchUpper) && text.length < 100)) {
			// Check if this element or its children are visible
			const rect = elem.getBoundingClientRect();
			if (rect.width > 0 && rect.height > 0) {
				candidates.push(elem);
			}
		}
	}

	console.log('[ClickByText] Found', candidates.length, 'visible elements with matching text');

	// Sort candidates by text length (prefer exact matches and smaller elements)
	candidates.sort((a, b) => {
		const aText = a.textContent?.trim() || '';
		const bText = b.textContent?.trim() || '';
		return aText.length - bText.length;
	});

	// Click the best match (smallest element with the text)
	if (candidates.length > 0) {
		const elem = candidates[0];
		const text = elem.textContent?.trim() || '';

		console.log('[ClickByText] Clicking best match:', elem.tagName, elem.className, text);

		// Scroll into view
		elem.scrollIntoView({ behavior: 'instant', block: 'center' });

		// Click it
		elem.click();

		// Also dispatch mouse event
		const clickEvent = new MouseEvent('click', {
			view: window,
			bubbles: true,
			cancelable: true
		});
		elem.dispatchEvent(clickEvent);

		console.log('[ClickByText] Clicked successfully');
		return JSON.stringify({
			success: true,
			element: elem.tagName,
			text: text,
			className: elem.className
		});
	}

	console.log('[ClickByText] No matching element found for:', searchText);
	return JSON.stringify({
		success: false,
		reason: 'no_matching_element',
		searched: searchText,
		candidatesFound: candidates.length
	});
})();
`, buttonText)

	var resultJSON string
	err := chromedp.Run(v.ctx, chromedp.Evaluate(script, &resultJSON))
	if err != nil {
		return fmt.Errorf("failed to execute click: %w", err)
	}

	var result struct {
		Success   bool   `json:"success"`
		Reason    string `json:"reason"`
		Element   string `json:"element"`
		Text      string `json:"text"`
		ClassName string `json:"className"`
	}

	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("failed to parse click result: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("click failed: %s", result.Reason)
	}

	return nil
}

// DetectAndClickStartButton combines vision detection with DOM clicking
func (v *VisionDOMDetector) DetectAndClickStartButton(screenshot *Screenshot) error {
	// Get button text description from vision
	buttonText, err := v.DetectStartButtonDescription(screenshot)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// Find and click button by text
	if err := v.ClickButtonByText(buttonText); err != nil {
		return fmt.Errorf("click failed: %w", err)
	}

	return nil
}

// GameplayAction represents an action suggested by vision AI
type GameplayAction struct {
	GameStarted  bool   // Is the game actively playing?
	ActionNeeded bool   // Does something need to be clicked?
	ButtonText   string // Text of button to click (if ActionNeeded)
	Description  string // Description of what needs to happen
	ClickX       int    // X coordinate to click (for canvas-rendered buttons)
	ClickY       int    // Y coordinate to click (for canvas-rendered buttons)
}

// DetectGameplayState analyzes screenshot to determine if game has started or if action is needed
func (v *VisionDOMDetector) DetectGameplayState(screenshot *Screenshot) (*GameplayAction, error) {
	// Encode screenshot to base64
	imageBase64 := base64.StdEncoding.EncodeToString(screenshot.Data)

	// Create vision request asking if game is playing or if action needed
	prompt := `Analyze this game screenshot and determine:
1. Is the game actively playing (gameplay visible, not a menu/splash screen)?
2. If not playing, is there a button or element that needs to be clicked to start/continue?
3. If a button needs to be clicked, estimate its CENTER coordinates in pixels from the top-left corner.

Respond in JSON format:
{
  "game_started": true/false,
  "action_needed": true/false,
  "button_text": "exact text on button to click" (if action_needed is true),
  "click_x": pixel x coordinate of button center (if action_needed is true),
  "click_y": pixel y coordinate of button center (if action_needed is true),
  "description": "brief description of what you see"
}

Examples:
- If showing "PLAY" button at center: {"game_started": false, "action_needed": true, "button_text": "PLAY", "click_x": 640, "click_y": 400, "description": "Main menu with PLAY button"}
- If showing level "1" button: {"game_started": false, "action_needed": true, "button_text": "1", "click_x": 200, "click_y": 300, "description": "Level selection screen"}
- If showing active gameplay: {"game_started": true, "action_needed": false, "button_text": "", "click_x": 0, "click_y": 0, "description": "Game is actively playing"}
- If loading screen: {"game_started": false, "action_needed": false, "button_text": "", "click_x": 0, "click_y": 0, "description": "Loading screen"}`

	resp, err := v.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
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
			MaxTokens: 300,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("vision API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from vision API")
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Parse JSON response
	var result struct {
		GameStarted  bool   `json:"game_started"`
		ActionNeeded bool   `json:"action_needed"`
		ButtonText   string `json:"button_text"`
		ClickX       int    `json:"click_x"`
		ClickY       int    `json:"click_y"`
		Description  string `json:"description"`
	}

	// Handle markdown code fences if present
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimPrefix(responseText, "```")
		responseText = strings.TrimSuffix(responseText, "```")
		responseText = strings.TrimSpace(responseText)
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse vision response: %w (response: %s)", err, responseText)
	}

	// Log the full vision response for debugging
	log.Printf("[Vision] Raw response: %s", responseText)
	log.Printf("[Vision] Parsed - GameStarted: %v, ActionNeeded: %v, ButtonText: '%s', Coords: (%d, %d), Description: '%s'",
		result.GameStarted, result.ActionNeeded, result.ButtonText, result.ClickX, result.ClickY, result.Description)

	return &GameplayAction{
		GameStarted:  result.GameStarted,
		ActionNeeded: result.ActionNeeded,
		ButtonText:   result.ButtonText,
		ClickX:       result.ClickX,
		ClickY:       result.ClickY,
		Description:  result.Description,
	}, nil
}

// ClickAt clicks at specific pixel coordinates using chromedp
func (v *VisionDOMDetector) ClickAt(x, y int) error {
	// JavaScript to click at specific coordinates with full event sequence
	script := fmt.Sprintf(`
(function() {
    console.log('[VisionClick] Clicking at coordinates:', %d, %d);

    // Get element at coordinates
    const element = document.elementFromPoint(%d, %d);
    console.log('[VisionClick] Element at point:', element?.tagName, element?.className, element?.id);

    if (!element) {
        return JSON.stringify({ success: false, reason: 'no_element_at_coordinates' });
    }

    // If clicking on a canvas, dispatch full mouse event sequence
    if (element.tagName === 'CANVAS') {
        const canvas = element;
        const rect = canvas.getBoundingClientRect();

        console.log('[VisionClick] Canvas detected');
        console.log('[VisionClick] Canvas position:', {
            left: rect.left,
            top: rect.top,
            width: rect.width,
            height: rect.height
        });
        console.log('[VisionClick] Canvas internal size:', {
            width: canvas.width,
            height: canvas.height
        });

        // Dispatch mousedown event (many games use this instead of click)
        const mousedownEvent = new MouseEvent('mousedown', {
            view: window,
            bubbles: true,
            cancelable: true,
            clientX: %d,
            clientY: %d,
            button: 0  // Left mouse button
        });

        const mouseupEvent = new MouseEvent('mouseup', {
            view: window,
            bubbles: true,
            cancelable: true,
            clientX: %d,
            clientY: %d,
            button: 0
        });

        const clickEvent = new MouseEvent('click', {
            view: window,
            bubbles: true,
            cancelable: true,
            clientX: %d,
            clientY: %d
        });

        // Dispatch all three events in order (simulates real mouse interaction)
        canvas.dispatchEvent(mousedownEvent);
        canvas.dispatchEvent(mouseupEvent);
        canvas.dispatchEvent(clickEvent);

        // Also try pointer events (some games use Pointer Events API)
        const pointerdownEvent = new PointerEvent('pointerdown', {
            view: window,
            bubbles: true,
            cancelable: true,
            clientX: %d,
            clientY: %d,
            button: 0,
            pointerType: 'mouse'
        });

        const pointerupEvent = new PointerEvent('pointerup', {
            view: window,
            bubbles: true,
            cancelable: true,
            clientX: %d,
            clientY: %d,
            button: 0,
            pointerType: 'mouse'
        });

        canvas.dispatchEvent(pointerdownEvent);
        canvas.dispatchEvent(pointerupEvent);

        console.log('[VisionClick] Dispatched mouse and pointer events to canvas');

        return JSON.stringify({
            success: true,
            element: 'CANVAS',
            canvasId: canvas.id,
            canvasSize: { width: canvas.width, height: canvas.height },
            cssSize: { width: rect.width, height: rect.height },
            clickPosition: { x: %d, y: %d }
        });
    }

    // For non-canvas elements, use simple click
    element.click();
    element.dispatchEvent(new MouseEvent('click', {
        view: window,
        bubbles: true,
        cancelable: true,
        clientX: %d,
        clientY: %d
    }));

    console.log('[VisionClick] Clicked element');

    return JSON.stringify({
        success: true,
        element: element.tagName,
        className: element.className,
        id: element.id
    });
})();
`, x, y, x, y, x, y, x, y, x, y, x, y, x, y, x, y, x, y)

	var resultJSON string
	err := chromedp.Run(v.ctx, chromedp.Evaluate(script, &resultJSON))
	if err != nil {
		return fmt.Errorf("failed to execute click: %w", err)
	}

	// Parse result as generic map to handle both canvas and non-canvas responses
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("failed to parse click result: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		return fmt.Errorf("click failed: %v", result)
	}

	return nil
}

// InspectCanvasCoordinates logs detailed canvas dimension and coordinate information
func (v *VisionDOMDetector) InspectCanvasCoordinates() error {
	script := `
(function() {
    const canvas = document.querySelector('canvas');
    if (!canvas) {
        return JSON.stringify({ found: false });
    }

    const rect = canvas.getBoundingClientRect();
    const viewport = {
        width: window.innerWidth,
        height: window.innerHeight
    };

    return JSON.stringify({
        found: true,
        canvas: {
            internalWidth: canvas.width,
            internalHeight: canvas.height,
            cssWidth: rect.width,
            cssHeight: rect.height,
            position: {
                left: rect.left,
                top: rect.top,
                right: rect.right,
                bottom: rect.bottom
            }
        },
        viewport: viewport,
        scaleFactor: {
            x: canvas.width / rect.width,
            y: canvas.height / rect.height
        }
    });
})();
`

	var resultJSON string
	err := chromedp.Run(v.ctx, chromedp.Evaluate(script, &resultJSON))
	if err != nil {
		return fmt.Errorf("failed to inspect canvas: %w", err)
	}

	var result struct {
		Found bool `json:"found"`
		Canvas struct {
			InternalWidth  float64 `json:"internalWidth"`
			InternalHeight float64 `json:"internalHeight"`
			CSSWidth       float64 `json:"cssWidth"`
			CSSHeight      float64 `json:"cssHeight"`
			Position       struct {
				Left   float64 `json:"left"`
				Top    float64 `json:"top"`
				Right  float64 `json:"right"`
				Bottom float64 `json:"bottom"`
			} `json:"position"`
		} `json:"canvas"`
		Viewport struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"viewport"`
		ScaleFactor struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"scaleFactor"`
	}

	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("failed to parse canvas inspection result: %w", err)
	}

	if !result.Found {
		return fmt.Errorf("no canvas element found")
	}

	return nil
}
