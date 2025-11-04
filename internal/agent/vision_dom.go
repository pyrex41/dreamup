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
