package agent

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// MouseAction represents different types of mouse interactions
type MouseAction string

const (
	MouseActionClick MouseAction = "click"
	MouseActionDrag  MouseAction = "drag"
)

// PerformRandomClick clicks at a random position in the game area
// Avoids top navigation (rows 1-3) and edges
func PerformRandomClick(ctx context.Context, screenWidth, screenHeight int) error {
	// Click in center 60% of screen to avoid nav/ads
	// Avoid top 25% (rows 1-3 in 12-row grid)
	minX := int(float64(screenWidth) * 0.2)   // 20% from left
	maxX := int(float64(screenWidth) * 0.8)   // 80% from left
	minY := int(float64(screenHeight) * 0.25) // 25% from top (skip nav)
	maxY := int(float64(screenHeight) * 0.8)  // 80% from top

	x := minX + rand.Intn(maxX-minX)
	y := minY + rand.Intn(maxY-minY)

	log.Printf("[Mouse] Random click at (%d, %d)", x, y)

	// Use chromedp's MouseClickXY for consistent clicking
	err := chromedp.Run(ctx, chromedp.MouseClickXY(float64(x), float64(y)))
	if err != nil {
		return fmt.Errorf("random click failed: %w", err)
	}

	return nil
}

// DragPattern represents different drag movement patterns
type DragPattern string

const (
	DragPatternHorizontalLeft  DragPattern = "horizontal-left"  // Slingshot style (left drag)
	DragPatternHorizontalRight DragPattern = "horizontal-right" // Right swipe
	DragPatternVerticalUp      DragPattern = "vertical-up"      // Upward swipe
	DragPatternVerticalDown    DragPattern = "vertical-down"    // Downward swipe
	DragPatternDiagonal        DragPattern = "diagonal"         // Diagonal drag
)

// PerformRandomDrag performs a drag gesture with the specified pattern
func PerformRandomDrag(ctx context.Context, pattern DragPattern, screenWidth, screenHeight int) error {
	// For slingshot-style games, start on LEFT side where slingshot typically is
	// Start in left 20-30% of screen (slingshot area)
	var startX, startY int
	var endX, endY int
	dragDistance := 150 // pixels to drag

	if pattern == DragPatternHorizontalLeft {
		// Angry Birds slingshot: Start at left side (bird/slingshot position)
		slingshotMinX := int(float64(screenWidth) * 0.15) // 15% from left
		slingshotMaxX := int(float64(screenWidth) * 0.30) // 30% from left
		slingshotMinY := int(float64(screenHeight) * 0.40) // Middle-ish vertically
		slingshotMaxY := int(float64(screenHeight) * 0.60)

		startX = slingshotMinX + rand.Intn(slingshotMaxX-slingshotMinX)
		startY = slingshotMinY + rand.Intn(slingshotMaxY-slingshotMinY)
	} else {
		// Other patterns: use center area
		centerMinX := int(float64(screenWidth) * 0.4)
		centerMaxX := int(float64(screenWidth) * 0.6)
		centerMinY := int(float64(screenHeight) * 0.4)
		centerMaxY := int(float64(screenHeight) * 0.6)

		startX = centerMinX + rand.Intn(centerMaxX-centerMinX)
		startY = centerMinY + rand.Intn(centerMaxY-centerMinY)
	}

	// Calculate end position based on pattern
	switch pattern {
	case DragPatternHorizontalLeft:
		// Drag left to pull back slingshot (Angry Birds style)
		endX = startX - dragDistance
		endY = startY
	case DragPatternHorizontalRight:
		endX = startX + dragDistance
		endY = startY
	case DragPatternVerticalUp:
		endX = startX
		endY = startY - dragDistance
	case DragPatternVerticalDown:
		endX = startX
		endY = startY + dragDistance
	case DragPatternDiagonal:
		endX = startX - dragDistance/2
		endY = startY - dragDistance/2
	default:
		return fmt.Errorf("unknown drag pattern: %s", pattern)
	}

	// Clamp to screen bounds
	if endX < 0 {
		endX = 50
	}
	if endX > screenWidth {
		endX = screenWidth - 50
	}
	if endY < 0 {
		endY = 50
	}
	if endY > screenHeight {
		endY = screenHeight - 50
	}

	log.Printf("[Mouse] Drag %s from (%d,%d) to (%d,%d)", pattern, startX, startY, endX, endY)

	// Perform drag using chromedp CDP events
	err := PerformDrag(ctx, startX, startY, endX, endY, 300*time.Millisecond, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("drag %s failed: %w", pattern, err)
	}

	return nil
}

// PerformDrag executes a mouse drag from (startX, startY) to (endX, endY)
func PerformDrag(ctx context.Context, startX, startY, endX, endY int, duration, holdDuration time.Duration) error {
	// Mouse press at start position
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MousePressed, float64(startX), float64(startY)).
			WithButton(input.Left).
			WithClickCount(1).
			Do(ctx)
	}))
	if err != nil {
		return fmt.Errorf("mouse press failed: %w", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Mouse move to end position (with smooth interpolation)
	steps := 10
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := float64(startX) + float64(endX-startX)*t
		y := float64(startY) + float64(endY-startY)*t

		err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			return input.DispatchMouseEvent(input.MouseMoved, x, y).Do(ctx)
		}))
		if err != nil {
			return fmt.Errorf("mouse move failed at step %d: %w", i, err)
		}

		time.Sleep(duration / time.Duration(steps))
	}

	// Hold at end position
	time.Sleep(holdDuration)

	// Mouse release
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MouseReleased, float64(endX), float64(endY)).
			WithButton(input.Left).
			WithClickCount(1).
			Do(ctx)
	}))
	if err != nil {
		return fmt.Errorf("mouse release failed: %w", err)
	}

	return nil
}
