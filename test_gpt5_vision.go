package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
)

func main() {
	// Read the image
	imageData, err := os.ReadFile("gameplay.png")
	if err != nil {
		log.Fatalf("Failed to read image: %v", err)
	}

	// Encode to base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)
	log.Printf("Image size: %d bytes, base64 size: %d chars", len(imageData), len(imageBase64))

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	client := openai.NewClient(apiKey)

	// Simple prompt - just ask what's in the image
	prompt := `Analyze this Angry Birds game screenshot. Describe what you see and whether the game is:
1. At the main menu (with PLAY button)
2. At level selection screen (with numbered level buttons)
3. In active gameplay (slingshot, birds, pigs visible)
4. Loading screen

Keep your response under 100 words.`

	log.Printf("Calling GPT-5 with simple prompt...")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "gpt-5-2025-08-07",
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
			MaxCompletionTokens: 300,
		},
	)

	duration := time.Since(startTime)
	log.Printf("API call took: %v", duration)

	if err != nil {
		log.Fatalf("API error: %v", err)
	}

	if len(resp.Choices) == 0 {
		log.Fatal("No response from API")
	}

	log.Printf("FinishReason: %s", resp.Choices[0].FinishReason)
	log.Printf("Response:\n%s", resp.Choices[0].Message.Content)
}
