package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserManager manages browser lifecycle and navigation
type BrowserManager struct {
	allocCtx   context.Context
	allocCancel context.CancelFunc
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewBrowserManager creates a new browser manager
func NewBrowserManager(headless bool) (*BrowserManager, error) {
	// Create allocator context with Chrome
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", headless), // Only disable GPU in headless mode
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		// Block common ad and tracking domains
		chromedp.Flag("host-rules", "MAP *.doubleclick.net 127.0.0.1, MAP *.googlesyndication.com 127.0.0.1, MAP *.googleadservices.com 127.0.0.1, MAP *.google-analytics.com 127.0.0.1"),
		// Disable popup blocking to ensure game loads properly
		chromedp.Flag("disable-popup-blocking", false),
		// Hide automation detection
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// Create browser context
	ctx, cancel := chromedp.NewContext(allocCtx)

	bm := &BrowserManager{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		ctx:         ctx,
		cancel:      cancel,
	}

	return bm, nil
}

// Close shuts down the browser and cleans up resources
func (bm *BrowserManager) Close() {
	if bm.cancel != nil {
		bm.cancel()
	}
	if bm.allocCancel != nil {
		bm.allocCancel()
	}
}

// GetContext returns the browser context for running chromedp tasks
func (bm *BrowserManager) GetContext() context.Context {
	return bm.ctx
}

// Navigate navigates to the specified URL and waits for DOMContentLoaded
func (bm *BrowserManager) Navigate(url string) error {
	err := chromedp.Run(bm.ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}
	return nil
}

// NavigateWithTimeout navigates to URL with a specific timeout
func (bm *BrowserManager) NavigateWithTimeout(url string, timeout time.Duration) error {
	// Create a timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(bm.ctx, timeout)
	defer timeoutCancel()

	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)

	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("timeout after %v while loading %s", timeout, url)
		}
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}
	return nil
}

// LoadGame navigates to a game URL with 45-second timeout and waits for successful render
func (bm *BrowserManager) LoadGame(url string) error {
	const gameLoadTimeout = 45 * time.Second
	return bm.NavigateWithTimeout(url, gameLoadTimeout)
}

// RemoveAdsAndCookieConsent injects JavaScript to remove ad elements and handle cookie consent
func (bm *BrowserManager) RemoveAdsAndCookieConsent() error {
	script := `
(function() {
    console.log('[AdBlocker] Running ad removal and cookie consent handler...');

    // Remove common ad elements
    const adSelectors = [
        // Generic ad containers
        '[id*="ad-"]', '[id*="ads-"]', '[class*="ad-"]', '[class*="ads-"]',
        '[id*="banner"]', '[class*="banner"]',
        '[id*="sponsor"]', '[class*="sponsor"]',
        // Ad iframes
        'iframe[src*="doubleclick"]', 'iframe[src*="googlesyndication"]',
        'iframe[src*="advertising"]', 'iframe[src*="/ads/"]',
        // Specific ad networks
        '.adsbygoogle', '#aswift', '[id^="google_ads"]',
        // Video ads
        '[class*="video-ad"]', '[id*="video-ad"]',
    ];

    let removedCount = 0;
    adSelectors.forEach(selector => {
        try {
            const elements = document.querySelectorAll(selector);
            elements.forEach(el => {
                // Don't remove canvas or game container elements
                if (!el.querySelector('canvas') && !el.closest('[id*="game"]') && !el.closest('[class*="game"]')) {
                    el.remove();
                    removedCount++;
                }
            });
        } catch (e) {
            // Ignore errors for invalid selectors
        }
    });

    console.log('[AdBlocker] Removed', removedCount, 'ad elements');

    // Handle cookie consent - be very specific to avoid clicking game links
    const cookieHandled = (function() {
        // Look for cookie consent containers first
        const cookieContainers = document.querySelectorAll('[id*="cookie"], [class*="cookie"], [id*="consent"], [class*="consent"], [id*="gdpr"], [class*="gdpr"]');

        if (cookieContainers.length === 0) {
            console.log('[CookieConsent] No cookie consent dialogs found');
            return false;
        }

        console.log('[CookieConsent] Found', cookieContainers.length, 'cookie consent containers');

        // Within those containers, look for accept buttons
        for (let container of cookieContainers) {
            const buttons = container.querySelectorAll('button, a[role="button"]');

            for (let btn of buttons) {
                const text = btn.textContent.toLowerCase().trim();
                const id = (btn.id || '').toLowerCase();
                const className = (btn.className || '').toLowerCase();

                // Very specific patterns for accept buttons, exclude play/game buttons
                const isAcceptButton = (
                    (text.includes('accept all') || text.includes('allow all') ||
                     text.includes('agree') || text === 'accept' || text === 'ok' ||
                     id.includes('accept') || className.includes('accept')) &&
                    !text.includes('play') && !text.includes('game') && !text.includes('start')
                );

                if (isAcceptButton) {
                    console.log('[CookieConsent] Clicking accept button:', text);
                    btn.click();
                    return true;
                }
            }
        }

        console.log('[CookieConsent] No accept button found in cookie containers');
        return false;
    })();

    return JSON.stringify({
        adsRemoved: removedCount,
        cookieHandled: cookieHandled
    });
})();
`

	var result string
	err := chromedp.Run(bm.ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return fmt.Errorf("failed to remove ads and handle cookies: %w", err)
	}

	return nil
}
