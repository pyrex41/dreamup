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
