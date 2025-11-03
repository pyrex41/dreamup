package evaluator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dreamup/qa-agent/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

// PlayabilityScore represents the evaluation result from the LLM
type PlayabilityScore struct {
	// OverallScore is the overall playability score (0-100)
	OverallScore int `json:"overall_score"`
	// LoadsCorrectly indicates if the game loaded without errors
	LoadsCorrectly bool `json:"loads_correctly"`
	// InteractivityScore rates how responsive the game is (0-100)
	InteractivityScore int `json:"interactivity_score"`
	// VisualQuality rates the visual presentation (0-100)
	VisualQuality int `json:"visual_quality"`
	// ErrorSeverity rates the severity of any errors found (0-100, 0=none)
	ErrorSeverity int `json:"error_severity"`
	// Reasoning explains the LLM's evaluation rationale
	Reasoning string `json:"reasoning"`
	// Issues lists specific problems found during evaluation
	Issues []string `json:"issues"`
	// Recommendations suggests improvements
	Recommendations []string `json:"recommendations"`
}

// GameEvaluator handles LLM-based game evaluation
type GameEvaluator struct {
	client *openai.Client
	model  string
}

// NewGameEvaluator creates a new game evaluator with OpenAI client
func NewGameEvaluator(apiKey string) (*GameEvaluator, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not provided and not found in environment")
		}
	}

	client := openai.NewClient(apiKey)

	return &GameEvaluator{
		client: client,
		model:  "gpt-4o", // GPT-4o has vision capabilities
	}, nil
}

// SetModel allows changing the model (useful for testing or using different models)
func (ge *GameEvaluator) SetModel(model string) {
	ge.model = model
}

// encodeScreenshotToBase64 encodes a screenshot to base64 string
func encodeScreenshotToBase64(screenshot *agent.Screenshot) (string, error) {
	if screenshot.Data == nil || len(screenshot.Data) == 0 {
		return "", fmt.Errorf("screenshot data is empty")
	}

	encoded := base64.StdEncoding.EncodeToString(screenshot.Data)
	return encoded, nil
}

// buildEvaluationPrompt constructs the prompt for LLM evaluation
func buildEvaluationPrompt(screenshots []*agent.Screenshot, logs []agent.ConsoleLog) string {
	prompt := `You are a QA expert evaluating a web-based game's playability. Analyze the provided screenshots and console logs to assess the game's quality.

Evaluation Criteria:
1. **Loads Correctly**: Did the game load without critical errors?
2. **Interactivity**: Does the game appear responsive and functional?
3. **Visual Quality**: Are visuals rendering correctly (no broken images, proper layout)?
4. **Errors**: Are there console errors that impact gameplay?

Screenshots Context:
`

	for i, screenshot := range screenshots {
		prompt += fmt.Sprintf("- Image %d: %s phase (captured at %s)\n",
			i+1,
			screenshot.Context,
			screenshot.Timestamp.Format("15:04:05"),
		)
	}

	prompt += "\nConsole Logs Summary:\n"

	if len(logs) == 0 {
		prompt += "- No console logs captured\n"
	} else {
		errorCount := 0
		warningCount := 0
		for _, log := range logs {
			if log.Level == agent.LogLevelError {
				errorCount++
			} else if log.Level == agent.LogLevelWarning {
				warningCount++
			}
		}

		prompt += fmt.Sprintf("- Total logs: %d\n", len(logs))
		prompt += fmt.Sprintf("- Errors: %d\n", errorCount)
		prompt += fmt.Sprintf("- Warnings: %d\n", warningCount)

		// Include first few errors for context
		if errorCount > 0 {
			prompt += "\nSample Errors:\n"
			count := 0
			for _, log := range logs {
				if log.Level == agent.LogLevelError && count < 3 {
					prompt += fmt.Sprintf("- %s\n", log.Message)
					count++
				}
			}
		}
	}

	prompt += `
Provide your evaluation as a JSON object with this structure:
{
  "overall_score": <0-100>,
  "loads_correctly": <true/false>,
  "interactivity_score": <0-100>,
  "visual_quality": <0-100>,
  "error_severity": <0-100, where 0=no errors, 100=critical errors>,
  "reasoning": "<explanation of scores>",
  "issues": ["<issue 1>", "<issue 2>"],
  "recommendations": ["<recommendation 1>", "<recommendation 2>"]
}

Analyze the images and logs carefully, then respond with ONLY the JSON object.`

	return prompt
}

// EvaluateGame evaluates a game using screenshots and console logs
func (ge *GameEvaluator) EvaluateGame(ctx context.Context, screenshots []*agent.Screenshot, logs []agent.ConsoleLog) (*PlayabilityScore, error) {
	if len(screenshots) == 0 {
		return nil, fmt.Errorf("no screenshots provided for evaluation")
	}

	// Build prompt
	textPrompt := buildEvaluationPrompt(screenshots, logs)

	// Build message content with text and images
	messageParts := []openai.ChatMessagePart{
		{
			Type: openai.ChatMessagePartTypeText,
			Text: textPrompt,
		},
	}

	// Add up to 5 screenshots as images (GPT-4 Vision limit)
	maxImages := 5
	if len(screenshots) > maxImages {
		screenshots = screenshots[:maxImages]
	}

	for _, screenshot := range screenshots {
		base64Image, err := encodeScreenshotToBase64(screenshot)
		if err != nil {
			return nil, fmt.Errorf("failed to encode screenshot: %w", err)
		}

		messageParts = append(messageParts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    fmt.Sprintf("data:image/png;base64,%s", base64Image),
				Detail: openai.ImageURLDetailAuto,
			},
		})
	}

	// Create chat completion request
	req := openai.ChatCompletionRequest{
		Model: ge.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:         openai.ChatMessageRoleUser,
				MultiContent: messageParts,
			},
		},
		MaxTokens:   1500,
		Temperature: 0.3, // Lower temperature for more consistent evaluations
	}

	// Call OpenAI API
	resp, err := ge.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned from API")
	}

	// Parse JSON response
	responseText := resp.Choices[0].Message.Content

	// Strip markdown code fences if present
	responseText = stripMarkdownCodeFence(responseText)

	var score PlayabilityScore
	if err := json.Unmarshal([]byte(responseText), &score); err != nil {
		// If JSON parsing fails, return error with the raw response for debugging
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w\nRaw response: %s", err, responseText)
	}

	return &score, nil
}

// stripMarkdownCodeFence removes markdown code fence wrappers from JSON responses
func stripMarkdownCodeFence(text string) string {
	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	// Remove ```json and ``` wrappers if present
	if strings.HasPrefix(text, "```json") {
		// Find the closing ```
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSpace(text)
		if idx := strings.Index(text, "```"); idx != -1 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		// Handle generic ``` fence
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSpace(text)
		if idx := strings.Index(text, "```"); idx != -1 {
			text = text[:idx]
		}
	}

	// Trim any remaining whitespace
	return strings.TrimSpace(text)
}

// SaveScoreToFile saves the playability score to a JSON file
func SaveScoreToFile(score *PlayabilityScore, filepath string) error {
	data, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal score: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write score to %s: %w", filepath, err)
	}

	return nil
}
