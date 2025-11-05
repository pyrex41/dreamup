package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	prompt := `Analyze this game screenshot carefully and determine:

1. Is the game actively playing (gameplay visible, not a menu/splash screen)?
2. If not playing, is there a button or element that needs to be clicked to start/continue?
3. If a button needs to be clicked, locate its EXACT CENTER coordinates.

CRITICAL INSTRUCTIONS FOR COORDINATES:
- The image is 1280x720 pixels (width x height)
- Measure from TOP-LEFT corner (0, 0)
- Find the VISUAL CENTER of the clickable button/element
- Measure button boundaries carefully: find top, bottom, left, right edges
- Calculate center as: X = (left + right) / 2, Y = (top + bottom) / 2
- For buttons in LOWER THIRD of screen (below game title), Y should be > 500
- For horizontally centered buttons, X should be near 640
- Be precise - clicking 50+ pixels off will fail

SPECIAL NOTE: Many game menu buttons appear BELOW the game title/logo.
If you see a button in the lower portion of the menu, estimate Y > 550.

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
- Angry Birds PLAY button (large orange button below red bird and title): {"game_started": false, "action_needed": true, "button_text": "PLAY", "click_x": 640, "click_y": 590, "description": "Angry Birds main menu with PLAY button"}
- PLAY button in lower-center: {"game_started": false, "action_needed": true, "button_text": "PLAY", "click_x": 640, "click_y": 580, "description": "Main menu with PLAY button below title"}
- START button very low: {"game_started": false, "action_needed": true, "button_text": "START", "click_x": 640, "click_y": 620, "description": "Main menu with START button at bottom"}
- Level "1" button on left: {"game_started": false, "action_needed": true, "button_text": "1", "click_x": 200, "click_y": 300, "description": "Level selection screen"}
- Active gameplay: {"game_started": true, "action_needed": false, "button_text": "", "click_x": 0, "click_y": 0, "description": "Game is actively playing"}
- Loading screen: {"game_started": false, "action_needed": false, "button_text": "", "click_x": 0, "click_y": 0, "description": "Loading screen"}`

	// ===== DETAILED LOGGING =====
	log.Printf("[Vision Request] ========================================")
	log.Printf("[Vision Request] Prompt being sent to LLM:")
	log.Printf("[Vision Request] %s", prompt)
	log.Printf("[Vision Request] Screenshot metadata: %dx%d, %d bytes", screenshot.Width, screenshot.Height, len(screenshot.Data))
	log.Printf("[Vision Request] Base64 image size: %d chars", len(imageBase64))
	log.Printf("[Vision Request] Model: %s", openai.GPT4o)
	log.Printf("[Vision Request] ========================================")

	// Create context with 15 second timeout (vision API with large images can be slow)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := v.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4o, // Using full GPT-4o for better spatial reasoning
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
		log.Printf("[Vision Response] ERROR: %v", err)
		return nil, fmt.Errorf("vision API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		log.Printf("[Vision Response] ERROR: No choices in response")
		return nil, fmt.Errorf("no response from vision API")
	}

	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)

	// ===== DETAILED RESPONSE LOGGING =====
	log.Printf("[Vision Response] ========================================")
	log.Printf("[Vision Response] Raw response from LLM:")
	log.Printf("[Vision Response] %s", responseText)
	log.Printf("[Vision Response] ========================================")

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
		log.Printf("[Vision Parse] ERROR: Failed to parse JSON: %v", err)
		log.Printf("[Vision Parse] Attempted to parse: %s", responseText)
		return nil, fmt.Errorf("failed to parse vision response: %w (response: %s)", err, responseText)
	}

	// Log the parsed results
	log.Printf("[Vision Parsed] GameStarted: %v, ActionNeeded: %v, ButtonText: '%s', Coords: (%d, %d), Description: '%s'",
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

// SaveScreenshotWithClickMarker saves a screenshot with a visual marker showing where we clicked
func SaveScreenshotWithClickMarker(screenshot *Screenshot, x, y int, label string) (string, error) {
	// Decode PNG image
	img, err := png.Decode(bytes.NewReader(screenshot.Data))
	if err != nil {
		return "", fmt.Errorf("failed to decode screenshot: %w", err)
	}

	// Create a new RGBA image we can draw on
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Draw a red circle at the click point
	red := color.RGBA{255, 0, 0, 255}
	radius := 20

	// Draw circle outline
	for angle := 0; angle < 360; angle++ {
		rad := float64(angle) * math.Pi / 180
		for r := radius - 3; r <= radius; r++ {
			dx := int(float64(r) * math.Cos(rad))
			dy := int(float64(r) * math.Sin(rad))
			px := x + dx
			py := y + dy
			if px >= 0 && px < bounds.Max.X && py >= 0 && py < bounds.Max.Y {
				rgba.Set(px, py, red)
			}
		}
	}

	// Draw crosshair
	for i := -25; i <= 25; i++ {
		// Horizontal line
		if x+i >= 0 && x+i < bounds.Max.X {
			rgba.Set(x+i, y, red)
		}
		// Vertical line
		if y+i >= 0 && y+i < bounds.Max.Y {
			rgba.Set(x, y+i, red)
		}
	}

	// Save to temp file
	filename := fmt.Sprintf("click_marker_%s_%d_%d.png", label, x, y)
	filepath := filepath.Join(os.TempDir(), filename)

	f, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create marker file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, rgba); err != nil {
		return "", fmt.Errorf("failed to encode marker image: %w", err)
	}

	return filepath, nil
}

// ClickAt clicks at specific pixel coordinates using native CDP mouse events
func (v *VisionDOMDetector) ClickAt(x, y int) error {
	// Use chromedp's native MouseClickXY for real browser input events
	// This sends actual Input.dispatchMouseEvent through Chrome DevTools Protocol,
	// which games properly respond to (unlike JavaScript dispatchEvent)
	log.Printf("[VisionClick] Screenshot coordinates: (%d, %d)", x, y)

	// CRITICAL: Transform coordinates from screenshot space to actual viewport space
	// The screenshot was taken at 1280x720, but the actual viewport might be different
	script := fmt.Sprintf(`
(function() {
	const screenshotWidth = 1280;
	const screenshotHeight = 720;
	const currentWidth = window.innerWidth;
	const currentHeight = window.innerHeight;

	const scaleX = currentWidth / screenshotWidth;
	const scaleY = currentHeight / screenshotHeight;

	const viewportX = Math.round(%d * scaleX);
	const viewportY = Math.round(%d * scaleY);

	console.log('[VisionClick] Screenshot:', %d, %d, 'Viewport:', currentWidth, 'x', currentHeight);
	console.log('[VisionClick] Scale:', scaleX, 'x', scaleY, 'Transformed:', viewportX, viewportY);

	return JSON.stringify({ x: viewportX, y: viewportY, scaleX: scaleX, scaleY: scaleY });
})();
`, x, y, x, y)

	var resultJSON string
	err := chromedp.Run(v.ctx, chromedp.Evaluate(script, &resultJSON))
	if err != nil {
		return fmt.Errorf("failed to calculate transformed coordinates: %w", err)
	}

	var result struct {
		X      int     `json:"x"`
		Y      int     `json:"y"`
		ScaleX float64 `json:"scaleX"`
		ScaleY float64 `json:"scaleY"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("failed to parse coordinate transformation: %w", err)
	}

	log.Printf("[VisionClick] Transformed coordinates: (%d, %d) with scale (%.2f, %.2f)",
		result.X, result.Y, result.ScaleX, result.ScaleY)

	// Now click at the TRANSFORMED coordinates
	err = chromedp.Run(v.ctx,
		chromedp.MouseClickXY(float64(result.X), float64(result.Y)),
	)

	if err != nil {
		return fmt.Errorf("CDP mouse click failed: %w", err)
	}

	log.Printf("[VisionClick] ✓ Successfully clicked using native CDP mouse event")
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
