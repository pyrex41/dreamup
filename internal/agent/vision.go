package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chromedp/chromedp"
	openai "github.com/sashabaranov/go-openai"
)

// VisionDetector uses GPT-4o vision to detect UI elements and determine click coordinates
type VisionDetector struct {
	ctx    context.Context
	client *openai.Client
}

// ClickTarget represents a detected clickable element with its coordinates
type ClickTarget struct {
	// X coordinate (0-1280 range based on screenshot width)
	X int
	// Y coordinate (0-720 range based on screenshot height)
	Y int
	// Description of what was detected (e.g., "Start Game button")
	Description string
	// Confidence level (0.0-1.0)
	Confidence float64
}

// NewVisionDetector creates a new vision-based UI detector
func NewVisionDetector(ctx context.Context) (*VisionDetector, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	client := openai.NewClient(apiKey)

	return &VisionDetector{
		ctx:    ctx,
		client: client,
	}, nil
}

// DetectStartButton uses GPT-4o vision to find the start button and return click coordinates
func (v *VisionDetector) DetectStartButton(screenshot *Screenshot) (*ClickTarget, error) {
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
							Text: `You are analyzing a game screenshot to find the start button or play button.

The screenshot resolution is 1280x720 pixels with origin (0,0) at TOP-LEFT corner.

CRITICAL: You MUST return the EXACT pixel coordinates where the button appears in the image.
- Measure from the TOP-LEFT corner (0,0)
- X increases going RIGHT
- Y increases going DOWN
- Return the CENTER point of the button

Please analyze the image and identify the start/play button. Return ONLY a JSON object with this exact format:
{
  "found": true/false,
  "x": exact_pixel_x_coordinate,
  "y": exact_pixel_y_coordinate,
  "description": "brief description of the button",
  "confidence": 0.0-1.0
}

If you cannot find a start button with high confidence, set "found" to false.

Look for:
- Buttons with text like "START", "PLAY", "START GAME", "PLAY NOW", "BEGIN"
- Prominent green/yellow/colored buttons
- Arrow buttons or play icons
- The most obvious interactive element to start gameplay

IMPORTANT:
- Count pixels carefully from top-left
- If button is in upper-left, x and y should be SMALL numbers (like 100-200)
- If button is in center, x should be near 640, y near 360
- If button is in bottom-right, x near 1280, y near 720
- DO NOT just guess the center - measure the actual button location`,
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
			MaxTokens: 300,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("vision API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from vision API")
	}

	// Parse JSON response
	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Remove markdown code blocks if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result struct {
		Found       bool    `json:"found"`
		X           int     `json:"x"`
		Y           int     `json:"y"`
		Description string  `json:"description"`
		Confidence  float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse vision response: %w (content: %s)", err, content)
	}

	if !result.Found {
		return nil, fmt.Errorf("no start button detected with sufficient confidence")
	}

	// Validate coordinates are within bounds (must be strictly less than width/height)
	// 1280x720 means valid coords are 0-1279 for X and 0-719 for Y
	if result.X < 0 || result.X >= screenshot.Width || result.Y < 0 || result.Y >= screenshot.Height {
		return nil, fmt.Errorf("detected coordinates out of bounds: (%d, %d) for viewport %dx%d",
			result.X, result.Y, screenshot.Width, screenshot.Height)
	}

	return &ClickTarget{
		X:           result.X,
		Y:           result.Y,
		Description: result.Description,
		Confidence:  result.Confidence,
	}, nil
}

// ClickAt clicks at specific pixel coordinates using chromedp
func (v *VisionDetector) ClickAt(x, y int) error {
	// JavaScript to click at specific coordinates
	script := fmt.Sprintf(`
(function() {
    console.log('[VisionClick] Screenshot coordinates:', %d, %d);
    console.log('[VisionClick] Viewport size:', window.innerWidth, 'x', window.innerHeight);

    // CRITICAL: The screenshot was taken with viewport set to 1280x720
    // We need to check if current viewport matches and calculate scale if needed
    const screenshotWidth = 1280;
    const screenshotHeight = 720;
    const currentWidth = window.innerWidth;
    const currentHeight = window.innerHeight;

    // Calculate viewport scale factor
    const scaleX = currentWidth / screenshotWidth;
    const scaleY = currentHeight / screenshotHeight;

    // Transform coordinates from screenshot space to current viewport space
    const viewportX = Math.round(%d * scaleX);
    const viewportY = Math.round(%d * scaleY);

    console.log('[VisionClick] Scale factors:', scaleX, 'x', scaleY);
    console.log('[VisionClick] Transformed viewport coordinates:', viewportX, viewportY);

    // Get element at coordinates
    const element = document.elementFromPoint(viewportX, viewportY);
    console.log('[VisionClick] Element at point:', element?.tagName, element?.className, element?.id);

    if (!element) {
        return JSON.stringify({ success: false, reason: 'no_element_at_coordinates' });
    }

    // Dispatch multiple event types for better compatibility
    // Some games (especially HTML5 canvas games) require touch events

    // 1. Mouse events (using transformed viewport coordinates)
    const mouseDown = new MouseEvent('mousedown', {
        view: window,
        bubbles: true,
        cancelable: true,
        clientX: viewportX,
        clientY: viewportY
    });

    const mouseUp = new MouseEvent('mouseup', {
        view: window,
        bubbles: true,
        cancelable: true,
        clientX: viewportX,
        clientY: viewportY
    });

    const clickEvent = new MouseEvent('click', {
        view: window,
        bubbles: true,
        cancelable: true,
        clientX: viewportX,
        clientY: viewportY
    });

    // 2. Touch events for mobile/canvas games
    const touch = new Touch({
        identifier: Date.now(),
        target: element,
        clientX: viewportX,
        clientY: viewportY,
        radiusX: 2.5,
        radiusY: 2.5,
        rotationAngle: 0,
        force: 1
    });

    const touchStart = new TouchEvent('touchstart', {
        cancelable: true,
        bubbles: true,
        touches: [touch],
        targetTouches: [touch],
        changedTouches: [touch]
    });

    const touchEnd = new TouchEvent('touchend', {
        cancelable: true,
        bubbles: true,
        touches: [],
        targetTouches: [],
        changedTouches: [touch]
    });

    // 3. Pointer events (modern standard)
    const pointerDown = new PointerEvent('pointerdown', {
        bubbles: true,
        cancelable: true,
        clientX: viewportX,
        clientY: viewportY,
        pointerId: 1,
        pointerType: 'mouse'
    });

    const pointerUp = new PointerEvent('pointerup', {
        bubbles: true,
        cancelable: true,
        clientX: viewportX,
        clientY: viewportY,
        pointerId: 1,
        pointerType: 'mouse'
    });

    // Dispatch all event types in proper order
    element.dispatchEvent(pointerDown);
    element.dispatchEvent(mouseDown);
    element.dispatchEvent(touchStart);

    // Native click
    element.click();

    element.dispatchEvent(touchEnd);
    element.dispatchEvent(mouseUp);
    element.dispatchEvent(pointerUp);
    element.dispatchEvent(clickEvent);

    console.log('[VisionClick] All click events dispatched (mouse, touch, pointer)');

    return JSON.stringify({
        success: true,
        element: element.tagName,
        className: element.className,
        id: element.id,
        screenshotCoords: { x: %d, y: %d },
        viewportCoords: { x: viewportX, y: viewportY },
        scaleFactor: { x: scaleX, y: scaleY }
    });
})();
`, x, y, x, y, x, y)

	var resultJSON string
	err := chromedp.Run(v.ctx, chromedp.Evaluate(script, &resultJSON))
	if err != nil {
		return fmt.Errorf("failed to execute click: %w", err)
	}

	var result struct {
		Success   bool   `json:"success"`
		Reason    string `json:"reason"`
		Element   string `json:"element"`
		ClassName string `json:"className"`
		ID        string `json:"id"`
	}

	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("failed to parse click result: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("click failed: %s", result.Reason)
	}

	return nil
}

// DetectAndClickStartButton combines detection and clicking in one method
func (v *VisionDetector) DetectAndClickStartButton(screenshot *Screenshot) (*ClickTarget, error) {
	// Detect button
	target, err := v.DetectStartButton(screenshot)
	if err != nil {
		return nil, fmt.Errorf("detection failed: %w", err)
	}

	// Click at detected coordinates
	if err := v.ClickAt(target.X, target.Y); err != nil {
		return target, fmt.Errorf("click failed: %w", err)
	}

	return target, nil
}
