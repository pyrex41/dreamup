package agent

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

// UIElement represents a detected UI element
type UIElement struct {
	// Selector is the CSS selector that uniquely identifies this element
	Selector string
	// Type is the type of UI element (button, canvas, input, etc.)
	Type UIElementType
	// Text is the text content of the element (if any)
	Text string
	// Visible indicates whether the element is currently visible
	Visible bool
	// Attributes contains additional element attributes
	Attributes map[string]string
}

// UIElementType represents the type of UI element
type UIElementType string

const (
	// UITypeButton represents a button element
	UITypeButton UIElementType = "button"
	// UITypeCanvas represents a canvas element (common for games)
	UITypeCanvas UIElementType = "canvas"
	// UITypeInput represents an input field
	UITypeInput UIElementType = "input"
	// UITypeLink represents a link/anchor element
	UITypeLink UIElementType = "link"
	// UITypeDiv represents a div container
	UITypeDiv UIElementType = "div"
)

// UIPattern represents a common UI pattern to detect
type UIPattern struct {
	// Name is a descriptive name for this pattern
	Name string
	// Selectors is a list of CSS selectors to try (in priority order)
	Selectors []string
	// Type is the expected element type
	Type UIElementType
	// Required indicates if this element is required for the game to function
	Required bool
}

// Common UI patterns for web games
var (
	// StartButtonPattern detects common start button patterns
	StartButtonPattern = UIPattern{
		Name: "Start Button",
		Selectors: []string{
			"button:contains('Start')",
			"button:contains('Play')",
			"button:contains('BEGIN')",
			"#start-button",
			"#play-button",
			".start-btn",
			".play-btn",
			"button.start",
			"button.play",
			"input[type='button'][value*='Start']",
			"input[type='button'][value*='Play']",
		},
		Type:     UITypeButton,
		Required: true,
	}

	// GameCanvasPattern detects game canvas elements
	GameCanvasPattern = UIPattern{
		Name: "Game Canvas",
		Selectors: []string{
			"canvas#game",
			"canvas#gameCanvas",
			"canvas.game",
			"canvas.game-canvas",
			"canvas[id*='game']",
			"canvas",
		},
		Type:     UITypeCanvas,
		Required: true,
	}

	// PauseButtonPattern detects pause button patterns
	PauseButtonPattern = UIPattern{
		Name: "Pause Button",
		Selectors: []string{
			"button:contains('Pause')",
			"#pause-button",
			".pause-btn",
			"button.pause",
		},
		Type:     UITypeButton,
		Required: false,
	}

	// ResetButtonPattern detects reset/restart button patterns
	ResetButtonPattern = UIPattern{
		Name: "Reset Button",
		Selectors: []string{
			"button:contains('Reset')",
			"button:contains('Restart')",
			"#reset-button",
			"#restart-button",
			".reset-btn",
			"button.reset",
		},
		Type:     UITypeButton,
		Required: false,
	}
)

// AllCommonPatterns returns all common UI patterns
func AllCommonPatterns() []UIPattern {
	return []UIPattern{
		StartButtonPattern,
		GameCanvasPattern,
		PauseButtonPattern,
		ResetButtonPattern,
	}
}

// UIDetector handles UI element detection
type UIDetector struct {
	ctx context.Context
}

// NewUIDetector creates a new UI detector
func NewUIDetector(ctx context.Context) *UIDetector {
	return &UIDetector{
		ctx: ctx,
	}
}

// DetectPattern attempts to detect a UI pattern and returns the matching element
func (d *UIDetector) DetectPattern(pattern UIPattern) (*UIElement, error) {
	for _, selector := range pattern.Selectors {
		element, err := d.DetectElement(selector, pattern.Type)
		if err == nil && element != nil {
			return element, nil
		}
	}

	return nil, fmt.Errorf("pattern '%s' not found", pattern.Name)
}

// DetectElement tries to detect a specific element by selector
func (d *UIDetector) DetectElement(selector string, elementType UIElementType) (*UIElement, error) {
	var nodes []*cdp.Node

	// Query for the element
	err := chromedp.Run(d.ctx,
		chromedp.Nodes(selector, &nodes, chromedp.ByQuery),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query selector '%s': %w", selector, err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no elements found for selector '%s'", selector)
	}

	// Use the first matching node
	node := nodes[0]

	// Check if element exists (simplified - if we found it via query, it's detectable)
	visible := true

	// Extract text content if available
	var text string
	chromedp.Run(d.ctx,
		chromedp.Text(selector, &text, chromedp.ByQuery),
	)

	element := &UIElement{
		Selector:   selector,
		Type:       elementType,
		Text:       text,
		Visible:    visible,
		Attributes: make(map[string]string),
	}

	// Extract useful attributes
	if len(node.Attributes) > 0 {
		for i := 0; i < len(node.Attributes)-1; i += 2 {
			key := node.Attributes[i]
			value := node.Attributes[i+1]
			element.Attributes[key] = value
		}
	}

	return element, nil
}

// DetectAllPatterns attempts to detect all common UI patterns
func (d *UIDetector) DetectAllPatterns() map[string]*UIElement {
	results := make(map[string]*UIElement)

	for _, pattern := range AllCommonPatterns() {
		element, err := d.DetectPattern(pattern)
		if err == nil {
			results[pattern.Name] = element
		}
	}

	return results
}

// FindBestStartButton attempts to find the most likely start button
func (d *UIDetector) FindBestStartButton() (string, error) {
	element, err := d.DetectPattern(StartButtonPattern)
	if err != nil {
		return "", err
	}

	return element.Selector, nil
}

// HasGameCanvas checks if a game canvas is present
func (d *UIDetector) HasGameCanvas() bool {
	_, err := d.DetectPattern(GameCanvasPattern)
	return err == nil
}

// GetGameCanvas returns the game canvas selector if found
func (d *UIDetector) GetGameCanvas() (string, error) {
	element, err := d.DetectPattern(GameCanvasPattern)
	if err != nil {
		return "", err
	}

	return element.Selector, nil
}
