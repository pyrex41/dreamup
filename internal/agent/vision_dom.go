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
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	openai "github.com/sashabaranov/go-openai"
)

// VisionDOMDetector uses GPT-4o vision to identify elements by description, then finds them via DOM
type VisionDOMDetector struct {
	ctx    context.Context
	client *openai.Client
}

// NewVisionDOMDetector creates a new vision-based DOM detector
func NewVisionDOMDetector(ctx context.Context) (*VisionDOMDetector, error) {
	// Use OpenAI (more accurate for spatial reasoning)
	// Groq's Llama 4 Scout is faster but less accurate with grid coordinates
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable required")
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
			MaxCompletionTokens: 500, // GPT-5 needs more tokens than GPT-4o
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
	GridCell     string // Grid cell reference (e.g., "J7") for vision-based clicking
}

// DetectGameplayState analyzes screenshot to determine if game has started or if action is needed
func (v *VisionDOMDetector) DetectGameplayState(screenshot *Screenshot, gameMechanics string) (*GameplayAction, error) {
	// Apply grid overlay to screenshot for more reliable coordinate detection
	// Using 20 columns (A-T) and 12 rows (1-12) = 64x60 pixel cells for 1280x720
	gridCols := 20
	gridRows := 12
	griddedScreenshot, err := AddGridOverlay(screenshot, gridCols, gridRows)
	if err != nil {
		log.Printf("[Vision Grid] Warning: Failed to add grid overlay, using original: %v", err)
		griddedScreenshot = screenshot
	} else {
		log.Printf("[Vision Grid] Grid overlay applied: %d columns x %d rows", gridCols, gridRows)
	}

	// Encode screenshot with grid to base64
	imageBase64 := base64.StdEncoding.EncodeToString(griddedScreenshot.Data)

	// Build game mechanics section if provided
	var mechanicsSection string
	if gameMechanics != "" {
		mechanicsSection = fmt.Sprintf("\n\nGAME MECHANICS:\n%s\n\nUse these mechanics to understand how to interact with the game once gameplay has started.\n", gameMechanics)
		log.Printf("[Vision Game Mechanics] Provided: %s", gameMechanics)
	}

	// Simplified prompt for GPT-5 (uses fewer tokens)
	prompt := fmt.Sprintf(`Game screenshot analysis. Grid overlay: %dx%d (A-%s, 1-%d).

Is game playing? If not, what button to click?
- ONLY click PLAY/START/level numbers (rows 7-12)
- IGNORE "MORE GAMES", top nav (rows 1-3)
- Angry Birds PLAY: use J10 or K10
%s
JSON response:
{"game_started": bool, "action_needed": bool, "button_text": "text", "grid_cell": "J10", "description": "brief"}

Examples:
- Menu: {"game_started": false, "action_needed": true, "button_text": "PLAY", "grid_cell": "J10", "description": "main menu"}
- Levels: {"game_started": false, "action_needed": true, "button_text": "1", "grid_cell": "D4", "description": "level select"}
- Playing: {"game_started": true, "action_needed": false, "button_text": "", "grid_cell": "", "description": "gameplay active"}`,
		gridCols, gridRows, string(rune('A'+gridCols-1)), gridRows, mechanicsSection)

	// ===== DETAILED LOGGING =====
	log.Printf("[Vision Request] ========================================")
	log.Printf("[Vision Request] Prompt being sent to LLM:")
	log.Printf("[Vision Request] %s", prompt)
	log.Printf("[Vision Request] Screenshot metadata: %dx%d, %d bytes", screenshot.Width, screenshot.Height, len(screenshot.Data))
	log.Printf("[Vision Request] Base64 image size: %d chars", len(imageBase64))
	// Use GPT-4o for better spatial accuracy
	modelName := openai.GPT4o

	log.Printf("[Vision Request] Model: %s", modelName)
	log.Printf("[Vision Request] ========================================")

	// Create context with 30 second timeout (vision API with large images can be slow)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := v.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: modelName,
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
			MaxCompletionTokens: 800, // GPT-5 needs more tokens than GPT-4o
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

	// Debug: log finish reason and refusal
	log.Printf("[Vision Debug] FinishReason: %s", resp.Choices[0].FinishReason)
	if resp.Choices[0].Message.Refusal != "" {
		log.Printf("[Vision Debug] Refusal: %s", resp.Choices[0].Message.Refusal)
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
		GridCell     string `json:"grid_cell"` // Grid-based coordinate (e.g., "J7")
		Description  string `json:"description"`
	}

	// Extract JSON from response (handle both plain JSON and markdown code fences)
	jsonText := responseText

	// Look for JSON in markdown code fences
	if strings.Contains(responseText, "```json") {
		start := strings.Index(responseText, "```json")
		end := strings.Index(responseText[start+7:], "```")
		if end != -1 {
			jsonText = responseText[start+7 : start+7+end]
			jsonText = strings.TrimSpace(jsonText)
		}
	} else if strings.Contains(responseText, "{") {
		// Find the JSON object in the response
		start := strings.Index(responseText, "{")
		end := strings.LastIndex(responseText, "}")
		if start != -1 && end != -1 && end > start {
			jsonText = responseText[start : end+1]
		}
	}

	jsonText = strings.TrimSpace(jsonText)

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		log.Printf("[Vision Parse] ERROR: Failed to parse JSON: %v", err)
		log.Printf("[Vision Parse] Attempted to parse: %s", jsonText)
		return nil, fmt.Errorf("failed to parse vision response: %w (response: %s)", err, jsonText)
	}

	// Convert grid cell to pixel coordinates
	var clickX, clickY int
	if result.ActionNeeded && result.GridCell != "" {
		// Parse grid cell (e.g., "J7" -> column="J", row=7)
		gridCell, parseErr := parseGridCell(result.GridCell)
		if parseErr != nil {
			log.Printf("[Vision Grid] Warning: Failed to parse grid cell '%s': %v", result.GridCell, parseErr)
			// Fall back to center of screen
			clickX = screenshot.Width / 2
			clickY = screenshot.Height / 2
		} else {
			clickX, clickY = gridCell.ToPixelCoordinates(gridCols, gridRows, screenshot.Width, screenshot.Height)
			log.Printf("[Vision Grid] Converted grid cell %s to pixel coordinates (%d, %d)", result.GridCell, clickX, clickY)
		}
	}

	// Log the parsed results
	log.Printf("[Vision Parsed] GameStarted: %v, ActionNeeded: %v, ButtonText: '%s', GridCell: '%s', Coords: (%d, %d), Description: '%s'",
		result.GameStarted, result.ActionNeeded, result.ButtonText, result.GridCell, clickX, clickY, result.Description)

	return &GameplayAction{
		GameStarted:  result.GameStarted,
		ActionNeeded: result.ActionNeeded,
		ButtonText:   result.ButtonText,
		ClickX:       clickX,
		ClickY:       clickY,
		GridCell:     result.GridCell,
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

// GridCell represents a cell in the coordinate grid system
type GridCell struct {
	Column string // A, B, C, etc.
	Row    int    // 1, 2, 3, etc.
}

// String returns the grid cell in "A1" format
func (g GridCell) String() string {
	return fmt.Sprintf("%s%d", g.Column, g.Row)
}

// ToPixelCoordinates converts a grid cell to pixel coordinates (returns center of cell)
func (g GridCell) ToPixelCoordinates(gridCols, gridRows, imageWidth, imageHeight int) (int, int) {
	// Convert column letter to index (A=0, B=1, etc.)
	colIndex := int(g.Column[0] - 'A')
	rowIndex := g.Row - 1 // Row numbers are 1-based

	// Calculate cell size
	cellWidth := float64(imageWidth) / float64(gridCols)
	cellHeight := float64(imageHeight) / float64(gridRows)

	// Calculate center of cell
	x := int(float64(colIndex)*cellWidth + cellWidth/2)
	y := int(float64(rowIndex)*cellHeight + cellHeight/2)

	return x, y
}

// parseGridCell parses a grid cell string like "J7" into a GridCell struct
func parseGridCell(cellStr string) (GridCell, error) {
	cellStr = strings.ToUpper(strings.TrimSpace(cellStr))
	if len(cellStr) < 2 {
		return GridCell{}, fmt.Errorf("invalid grid cell format: %s", cellStr)
	}

	// Extract column letter(s) - could be A-Z or AA, AB, etc.
	colEnd := 0
	for i, ch := range cellStr {
		if ch < 'A' || ch > 'Z' {
			colEnd = i
			break
		}
	}
	if colEnd == 0 {
		return GridCell{}, fmt.Errorf("no row number found in grid cell: %s", cellStr)
	}

	column := cellStr[:colEnd]
	rowStr := cellStr[colEnd:]

	// Parse row number
	row, err := fmt.Sscanf(rowStr, "%d", new(int))
	if err != nil || row == 0 {
		return GridCell{}, fmt.Errorf("invalid row number in grid cell: %s", cellStr)
	}

	var rowNum int
	fmt.Sscanf(rowStr, "%d", &rowNum)

	return GridCell{
		Column: column,
		Row:    rowNum,
	}, nil
}

// AddGridOverlay adds a labeled grid overlay to a screenshot
// gridCols and gridRows define the grid dimensions (e.g., 20x12 for 20 columns, 12 rows)
func AddGridOverlay(screenshot *Screenshot, gridCols, gridRows int) (*Screenshot, error) {
	// Decode PNG image
	img, err := png.Decode(bytes.NewReader(screenshot.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	// Create a new RGBA image we can draw on
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Grid color (semi-transparent yellow)
	gridColor := color.RGBA{255, 255, 0, 128}
	textColor := color.RGBA{255, 255, 0, 255}

	width := bounds.Max.X
	height := bounds.Max.Y

	cellWidth := float64(width) / float64(gridCols)
	cellHeight := float64(height) / float64(gridRows)

	// Draw vertical lines and column labels
	for col := 0; col <= gridCols; col++ {
		x := int(float64(col) * cellWidth)

		// Draw vertical line
		for y := 0; y < height; y++ {
			if x >= 0 && x < width {
				rgba.Set(x, y, gridColor)
			}
		}

		// Add column label (A, B, C, etc.) at top and bottom
		if col < gridCols {
			label := string(rune('A' + col))
			labelX := int(float64(col)*cellWidth + cellWidth/2)

			// Draw label at top
			drawString(rgba, labelX-3, 12, label, textColor)
			// Draw label at bottom
			drawString(rgba, labelX-3, height-5, label, textColor)
		}
	}

	// Draw horizontal lines and row labels
	for row := 0; row <= gridRows; row++ {
		y := int(float64(row) * cellHeight)

		// Draw horizontal line
		for x := 0; x < width; x++ {
			if y >= 0 && y < height {
				rgba.Set(x, y, gridColor)
			}
		}

		// Add row label (1, 2, 3, etc.) on left and right
		if row < gridRows {
			label := fmt.Sprintf("%d", row+1)
			labelY := int(float64(row)*cellHeight + cellHeight/2)

			// Draw label on left
			drawString(rgba, 5, labelY+4, label, textColor)
			// Draw label on right
			drawString(rgba, width-15, labelY+4, label, textColor)
		}
	}

	// Encode the modified image back to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, fmt.Errorf("failed to encode gridded image: %w", err)
	}

	// Create a new screenshot with the gridded image
	griddedScreenshot := &Screenshot{
		Context:   screenshot.Context,
		Timestamp: screenshot.Timestamp,
		Data:      buf.Bytes(),
		Width:     screenshot.Width,
		Height:    screenshot.Height,
	}

	return griddedScreenshot, nil
}

// drawString draws a string on an RGBA image at the given position
func drawString(img *image.RGBA, x, y int, label string, col color.Color) {
	point := fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
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
