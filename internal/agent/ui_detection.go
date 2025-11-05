package agent

import (
	"context"
	"encoding/json"
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

	// CookieConsentPattern detects cookie consent buttons
	CookieConsentPattern = UIPattern{
		Name: "Cookie Consent",
		Selectors: []string{
			// Common IDs and classes (most specific first)
			"#didomi-notice-agree-button", // Didomi CMP
			"button.didomi-button",
			"#accept-cookies",
			"#cookie-accept",
			"#acceptCookies",
			".accept-cookies",
			".cookie-accept",
			".consent-accept",
			"button.consent-accept",
			// CMP (Consent Management Platform) specific
			".fc-cta-consent", // OneTrust
			".fc-button-background", // OneTrust alternate
			"button.fc-button",
			".qc-cmp2-summary-buttons button", // Quantcast
			"#truste-consent-button", // TrustArc
			".evidon-banner-acceptbutton", // Evidon
			// Attribute-based selectors
			"button[title*='accept' i]",
			"button[title*='agree' i]",
			"button[aria-label*='accept' i]",
			"button[aria-label*='agree' i]",
			"button[aria-label*='consent' i]",
			"[data-testid='accept-cookies']",
			"[data-testid='cookie-accept']",
			"[data-testid='consent-accept']",
			// Generic patterns (last resort)
			"div[class*='cookie' i] button",
			"div[class*='consent' i] button",
			"div[id*='cookie' i] button",
			"div[id*='consent' i] button",
		},
		Type:     UITypeButton,
		Required: false,
	}
)

// AllCommonPatterns returns all common UI patterns
func AllCommonPatterns() []UIPattern {
	return []UIPattern{
		CookieConsentPattern,
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

// ClickStartButton attempts to find and click a start/play button
// Returns true if a button was found and clicked, false otherwise
func (d *UIDetector) ClickStartButton() (bool, error) {
	// Use JavaScript to find and click start/play buttons
	script := `
(function() {
	console.log('[StartButton] Starting detection...');

	// Try finding buttons by text content
	const buttons = document.querySelectorAll('button, a[role="button"], div[role="button"], a, span[role="button"], input[type="button"], input[type="submit"], div, span, img, area');
	console.log('[StartButton] Found', buttons.length, 'potential button elements');

	for (const btn of buttons) {
		const text = btn.textContent.toLowerCase().trim();
		const value = (btn.value || '').toLowerCase().trim();
		const alt = (btn.alt || '').toLowerCase().trim();
		const title = (btn.title || '').toLowerCase().trim();
		const ariaLabel = (btn.getAttribute('aria-label') || '').toLowerCase().trim();

		// Match common start/play button text
		if (text === 'play' || text === 'start' || text === 'begin' ||
		    text === 'play game' || text === 'start game' ||
		    text.includes('play now') || text.includes('start now') ||
		    value === 'play' || value === 'start' ||
		    alt === 'play' || alt === 'start' ||
		    title === 'play' || title === 'start' ||
		    ariaLabel === 'play' || ariaLabel === 'start') {
			// Check if element is visible
			const rect = btn.getBoundingClientRect();
			if (rect.width > 0 && rect.height > 0 && btn.offsetParent !== null) {
				console.log('[StartButton] Clicking button with text:', text || alt || title || ariaLabel);
				btn.click();
				return true;
			}
		}
	}

	// Try clicking canvas (many games start on canvas click)
	const canvas = document.querySelector('canvas');
	if (canvas && canvas.offsetParent !== null) {
		console.log('[StartButton] No button found, clicking canvas');
		canvas.click();
		return true;
	}

	console.log('[StartButton] No start button or canvas found');
	return false;
})();
`

	var clicked bool
	err := chromedp.Run(d.ctx,
		chromedp.Evaluate(script, &clicked),
	)

	if err != nil {
		return false, fmt.Errorf("failed to run start button script: %w", err)
	}

	return clicked, nil
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

// FindCookieConsentButton attempts to find a cookie consent button
func (d *UIDetector) FindCookieConsentButton() (string, error) {
	element, err := d.DetectPattern(CookieConsentPattern)
	if err != nil {
		return "", err
	}

	return element.Selector, nil
}

// HasCookieConsent checks if a cookie consent dialog is present
func (d *UIDetector) HasCookieConsent() bool {
	_, err := d.DetectPattern(CookieConsentPattern)
	return err == nil
}

// AcceptCookieConsent attempts to accept cookie consent if present
// Returns true if consent was found and clicked, false otherwise
func (d *UIDetector) AcceptCookieConsent() (bool, error) {
	// Use JavaScript to find and click cookie consent buttons
	// This is more reliable than CSS selectors with chromedp
	script := `
(function() {
	// Common consent button selectors and text patterns
	const selectors = [
		// CMPs
		'#didomi-notice-agree-button',
		'button.didomi-button',
		'.fc-cta-consent',
		'button[aria-label*="accept" i]',
		'button[aria-label*="agree" i]',
		'button[title*="accept" i]'
	];

	// Try specific selectors first
	for (const selector of selectors) {
		try {
			const btn = document.querySelector(selector);
			if (btn && btn.offsetParent !== null) {
				btn.click();
				return true;
			}
		} catch (e) {
			// Invalid selector, continue
			continue;
		}
	}

	// Try finding buttons by text content - be very aggressive
	const buttons = document.querySelectorAll('button, a[role="button"], div[role="button"], a, span[role="button"]');
	for (const btn of buttons) {
		const text = btn.textContent.toLowerCase().trim();
		// Match common consent text patterns
		if (text === 'accept all cookies' || text === 'accept all' ||
		    text === 'accept cookies' || text === 'i accept' ||
		    text.includes('accept all cookies') ||
		    text.includes('accept') && text.includes('cookies') ||
		    text.includes('accept') && text.includes('all') ||
		    text.includes('agree') || text.includes('consent') ||
		    text.includes('ok') || text.includes('got it') ||
		    text.includes('allow') || text.includes('continue') ||
		    text === 'j\'accepte') {
			// Check if button is visible
			if (btn.offsetParent !== null) {
				btn.click();
				return true;
			}
		}
	}

	// Try to check iframes for consent dialogs
	const iframes = document.querySelectorAll('iframe');
	for (const iframe of iframes) {
		try {
			const iframeDoc = iframe.contentDocument || iframe.contentWindow.document;
			const iframeButtons = iframeDoc.querySelectorAll('button, a[role="button"], div[role="button"]');
			for (const btn of iframeButtons) {
				const text = btn.textContent.toLowerCase().trim();
				if (text.includes('accept') || text.includes('agree') || text.includes('consent')) {
					btn.click();
					return true;
				}
			}
		} catch (e) {
			// Cross-origin iframe, skip
			continue;
		}
	}

	return false;
})();
`

	var clicked bool
	err := chromedp.Run(d.ctx,
		chromedp.Evaluate(script, &clicked),
	)

	if err != nil {
		return false, fmt.Errorf("failed to run cookie consent script: %w", err)
	}

	return clicked, nil
}

// FocusGameCanvas focuses the game canvas element to ensure it receives keyboard events
// Returns true if canvas was found and focused successfully
func (d *UIDetector) FocusGameCanvas() (bool, error) {
	script := `
(function() {
	console.log('[FocusGameCanvas] Starting canvas focus...');

	// Try to find canvas in main document first
	let canvas = document.querySelector('canvas');
	console.log('[FocusGameCanvas] Canvas in main document:', !!canvas);

	// If not found, check iframes
	if (!canvas) {
		const iframes = document.querySelectorAll('iframe');
		console.log('[FocusGameCanvas] Checking', iframes.length, 'iframes');
		for (let iframe of iframes) {
			try {
				const iframeCanvas = iframe.contentDocument?.querySelector('canvas');
				if (iframeCanvas) {
					console.log('[FocusGameCanvas] Found canvas in iframe');
					canvas = iframeCanvas;
					break;
				}
			} catch (e) {
				// CORS issue, can't access iframe
				console.log('[FocusGameCanvas] Cannot access iframe (CORS):', e.message);
			}
		}
	}

	if (!canvas) {
		console.log('[FocusGameCanvas] No canvas found anywhere');
		return JSON.stringify({ success: false, reason: 'no_canvas' });
	}

	// Make canvas focusable by setting tabindex
	canvas.setAttribute('tabindex', '0');
	console.log('[FocusGameCanvas] Set tabindex=0');

	// Focus the canvas element
	canvas.focus();
	console.log('[FocusGameCanvas] Called focus()');

	// Check if canvas is in an iframe
	const inIframe = canvas.ownerDocument !== document;
	console.log('[FocusGameCanvas] Canvas in iframe:', inIframe);

	// Verify focus was successful
	const activeElement = canvas.ownerDocument.activeElement;
	const isFocused = activeElement === canvas;
	console.log('[FocusGameCanvas] Is focused:', isFocused, 'Active element:', activeElement?.tagName);

	return JSON.stringify({
		success: isFocused,
		inIframe: inIframe,
		activeTag: activeElement?.tagName
	});
})();
`

	var resultJSON string
	err := chromedp.Run(d.ctx,
		chromedp.Evaluate(script, &resultJSON),
	)

	if err != nil {
		return false, fmt.Errorf("failed to focus game canvas: %w", err)
	}

	// Parse JSON result
	var result struct {
		Success   bool   `json:"success"`
		Reason    string `json:"reason"`
		InIframe  bool   `json:"inIframe"`
		ActiveTag string `json:"activeTag"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return false, fmt.Errorf("failed to parse focus result: %w", err)
	}

	if !result.Success {
		return false, fmt.Errorf("canvas focus failed: %s (inIframe=%v, activeTag=%s)",
			result.Reason, result.InIframe, result.ActiveTag)
	}

	return true, nil
}

// SendKeyboardEventToCanvas sends a keyboard event directly to the canvas element
// keyCode is the key to send (e.g., "ArrowUp", "ArrowDown", "Space", "w", "a", "s", "d")
// Returns true if the event was dispatched successfully
func (d *UIDetector) SendKeyboardEventToCanvas(keyCode string) (bool, error) {
	script := fmt.Sprintf(`
(function() {
	const canvas = document.querySelector('canvas');
	if (!canvas) {
		return false;
	}

	// Ensure canvas is focused
	if (document.activeElement !== canvas) {
		canvas.focus();
	}

	// Key mappings - must match real browser keyboard events
	const keyMappings = {
		'ArrowUp': { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 },
		'ArrowDown': { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 },
		'ArrowLeft': { key: 'ArrowLeft', code: 'ArrowLeft', keyCode: 37 },
		'ArrowRight': { key: 'ArrowRight', code: 'ArrowRight', keyCode: 39 },
		'Space': { key: ' ', code: 'Space', keyCode: 32 },
		'Enter': { key: 'Enter', code: 'Enter', keyCode: 13 },
		'Escape': { key: 'Escape', code: 'Escape', keyCode: 27 }
	};

	const inputKey = %q;
	const mapping = keyMappings[inputKey] || {
		key: inputKey,
		code: 'Key' + inputKey.toUpperCase(),
		keyCode: inputKey.charCodeAt(0)
	};

	// Create and dispatch keydown event to both canvas and window
	const keydownEvent = new KeyboardEvent('keydown', {
		key: mapping.key,
		code: mapping.code,
		keyCode: mapping.keyCode,
		which: mapping.keyCode,
		bubbles: true,
		cancelable: true,
		composed: true
	});

	canvas.dispatchEvent(keydownEvent);
	window.dispatchEvent(keydownEvent);
	document.dispatchEvent(keydownEvent);

	// Small delay between keydown and keyup
	setTimeout(function() {
		const keyupEvent = new KeyboardEvent('keyup', {
			key: mapping.key,
			code: mapping.code,
			keyCode: mapping.keyCode,
			which: mapping.keyCode,
			bubbles: true,
			cancelable: true,
			composed: true
		});

		canvas.dispatchEvent(keyupEvent);
		window.dispatchEvent(keyupEvent);
		document.dispatchEvent(keyupEvent);
	}, 50);

	return true;
})();
`, keyCode)

	var dispatched bool
	err := chromedp.Run(d.ctx,
		chromedp.Evaluate(script, &dispatched),
	)

	if err != nil {
		return false, fmt.Errorf("failed to send keyboard event %s: %w", keyCode, err)
	}

	return dispatched, nil
}

// SendKeyboardEventToWindow sends a keyboard event to window and document (for DOM-based games)
// This doesn't require a canvas element and works for games rendered with DOM elements
func (d *UIDetector) SendKeyboardEventToWindow(keyCode string) (bool, error) {
	script := fmt.Sprintf(`
(function() {
	console.log('[SendKeyToWindow] Sending key:', '%s');

	const keyMappings = {
		'ArrowUp': { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 },
		'ArrowDown': { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 },
		'ArrowLeft': { key: 'ArrowLeft', code: 'ArrowLeft', keyCode: 37 },
		'ArrowRight': { key: 'ArrowRight', code: 'ArrowRight', keyCode: 39 },
		' ': { key: ' ', code: 'Space', keyCode: 32 }
	};

	const inputKey = %q;
	const mapping = keyMappings[inputKey] || {
		key: inputKey,
		code: 'Key' + inputKey.toUpperCase(),
		keyCode: inputKey.charCodeAt(0)
	};

	// Create and dispatch keydown event to window and document
	const keydownEvent = new KeyboardEvent('keydown', {
		key: mapping.key,
		code: mapping.code,
		keyCode: mapping.keyCode,
		which: mapping.keyCode,
		bubbles: true,
		cancelable: true,
		composed: true
	});

	window.dispatchEvent(keydownEvent);
	document.dispatchEvent(keydownEvent);
	document.body?.dispatchEvent(keydownEvent);

	// Small delay between keydown and keyup
	setTimeout(function() {
		const keyupEvent = new KeyboardEvent('keyup', {
			key: mapping.key,
			code: mapping.code,
			keyCode: mapping.keyCode,
			which: mapping.keyCode,
			bubbles: true,
			cancelable: true,
			composed: true
		});

		window.dispatchEvent(keyupEvent);
		document.dispatchEvent(keyupEvent);
		document.body?.dispatchEvent(keyupEvent);
	}, 50);

	console.log('[SendKeyToWindow] Dispatched to window/document/body');
	return true;
})();
`, keyCode, keyCode)

	var dispatched bool
	err := chromedp.Run(d.ctx,
		chromedp.Evaluate(script, &dispatched),
	)

	if err != nil {
		return false, fmt.Errorf("failed to send keyboard event %s to window: %w", keyCode, err)
	}

	return dispatched, nil
}

// WaitForGameReady polls the canvas to check if it has been rendered (not blank)
// Returns true if canvas is ready, false if timeout reached
func (d *UIDetector) WaitForGameReady(timeoutSeconds int) (bool, error) {
	script := fmt.Sprintf(`
(function() {
	return new Promise(function(resolve) {
		const timeout = %d * 1000; // Convert to milliseconds
		const startTime = Date.now();
		const pollInterval = 100; // Check every 100ms for faster detection

		function checkCanvas() {
			const canvas = document.querySelector('canvas');
			if (!canvas) {
				if (Date.now() - startTime >= timeout) {
					resolve(false); // Timeout - no canvas found
					return;
				}
				setTimeout(checkCanvas, pollInterval);
				return;
			}

			// Check if canvas has been rendered (width and height are set)
			if (canvas.width > 0 && canvas.height > 0) {
				// Try to check if canvas has any content
				try {
					const ctx = canvas.getContext('2d');
					const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
					const data = imageData.data;

					// Check if canvas has any non-transparent pixels
					let hasContent = false;
					for (let i = 3; i < data.length; i += 4) {
						if (data[i] > 0) { // Alpha channel > 0
							hasContent = true;
							break;
						}
					}

					if (hasContent) {
						resolve(true); // Canvas is rendered and has content
						return;
					}
				} catch (e) {
					// Can't read canvas (CORS issue), assume it's ready if sized
					resolve(true);
					return;
				}
			}

			// Not ready yet, check again
			if (Date.now() - startTime >= timeout) {
				resolve(false); // Timeout
				return;
			}
			setTimeout(checkCanvas, pollInterval);
		}

		checkCanvas();
	});
})();
`, timeoutSeconds)

	var ready bool
	err := chromedp.Run(d.ctx,
		chromedp.Evaluate(script, &ready),
	)

	if err != nil {
		return false, fmt.Errorf("failed to wait for game ready: %w", err)
	}

	return ready, nil
}
