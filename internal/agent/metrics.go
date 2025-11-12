package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// PerformanceMetrics contains all collected performance metrics
type PerformanceMetrics struct {
	FPS              *FPSMetrics         `json:"fps"`
	LoadTime         *LoadTimeMetrics    `json:"load_time"`
	Accessibility    *AccessibilityReport `json:"accessibility"`
	CollectionTime   time.Time           `json:"collection_time"`
}

// FPSMetrics contains frame rate performance data
type FPSMetrics struct {
	AverageFPS  float64   `json:"average_fps"`
	MinFPS      float64   `json:"min_fps"`
	MaxFPS      float64   `json:"max_fps"`
	Samples     int       `json:"samples"`
	Duration    float64   `json:"duration_seconds"`
	Frames      []float64 `json:"frames,omitempty"`
}

// LoadTimeMetrics contains page load timing information
type LoadTimeMetrics struct {
	// Navigation Timing API metrics (all in milliseconds)
	DNSLookup            int64   `json:"dns_lookup_ms"`
	TCPConnection        int64   `json:"tcp_connection_ms"`
	ServerResponse       int64   `json:"server_response_ms"`
	PageDownload         int64   `json:"page_download_ms"`
	DOMContentLoaded     int64   `json:"dom_content_loaded_ms"`
	WindowLoad           int64   `json:"window_load_ms"`
	TotalLoadTime        int64   `json:"total_load_time_ms"`

	// Resource timing
	ResourceCount        int     `json:"resource_count"`
	LargestResourceSize  int64   `json:"largest_resource_bytes"`
	LargestResourceURL   string  `json:"largest_resource_url"`

	// Paint timing
	FirstPaint           float64 `json:"first_paint_ms"`
	FirstContentfulPaint float64 `json:"first_contentful_paint_ms"`
}

// AccessibilityReport contains WCAG compliance check results
type AccessibilityReport struct {
	Score          int                    `json:"score"` // 0-100
	ViolationCount int                    `json:"violation_count"`
	WarningCount   int                    `json:"warning_count"`
	PassCount      int                    `json:"pass_count"`
	Violations     []AccessibilityViolation `json:"violations"`
	Warnings       []AccessibilityViolation `json:"warnings,omitempty"`
	Summary        string                   `json:"summary"`
}

// AccessibilityViolation represents a single accessibility issue
type AccessibilityViolation struct {
	Rule        string   `json:"rule"`
	Impact      string   `json:"impact"` // critical, serious, moderate, minor
	Description string   `json:"description"`
	HelpURL     string   `json:"help_url"`
	Elements    []string `json:"elements"` // CSS selectors
	Count       int      `json:"count"`
}

// MetricsCollector handles collecting all performance metrics
type MetricsCollector struct {
	ctx context.Context
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(ctx context.Context) *MetricsCollector {
	return &MetricsCollector{
		ctx: ctx,
	}
}

// CollectAll collects all metrics (FPS, load time, accessibility)
func (mc *MetricsCollector) CollectAll() (*PerformanceMetrics, error) {
	metrics := &PerformanceMetrics{
		CollectionTime: time.Now(),
	}

	// Collect FPS metrics
	fps, err := mc.CollectFPS(3 * time.Second) // 3 second sampling
	if err != nil {
		// Log but don't fail - FPS might not be available for all pages
		fmt.Printf("Warning: Failed to collect FPS metrics: %v\n", err)
	} else {
		metrics.FPS = fps
	}

	// Collect load time metrics
	loadTime, err := mc.CollectLoadTime()
	if err != nil {
		fmt.Printf("Warning: Failed to collect load time metrics: %v\n", err)
	} else {
		metrics.LoadTime = loadTime
	}

	// Collect accessibility metrics
	accessibility, err := mc.CollectAccessibility()
	if err != nil {
		fmt.Printf("Warning: Failed to collect accessibility metrics: %v\n", err)
	} else {
		metrics.Accessibility = accessibility
	}

	return metrics, nil
}

// CollectFPS monitors frame rate over a specified duration
func (mc *MetricsCollector) CollectFPS(duration time.Duration) (*FPSMetrics, error) {
	script := fmt.Sprintf(`
(function() {
    return new Promise(function(resolve) {
        const frames = [];
        let lastTime = performance.now();
        let frameCount = 0;
        const duration = %d; // milliseconds
        const startTime = performance.now();

        function measureFrame(currentTime) {
            const elapsed = performance.now() - startTime;

            if (elapsed >= duration) {
                // Calculate statistics
                const avgFPS = frames.length > 0 ? frames.reduce((a, b) => a + b, 0) / frames.length : 0;
                const minFPS = frames.length > 0 ? Math.min(...frames) : 0;
                const maxFPS = frames.length > 0 ? Math.max(...frames) : 0;

                resolve({
                    averageFPS: avgFPS,
                    minFPS: minFPS,
                    maxFPS: maxFPS,
                    samples: frames.length,
                    duration: elapsed / 1000,
                    frames: frames
                });
                return;
            }

            // Calculate FPS for this frame
            const delta = currentTime - lastTime;
            if (delta > 0) {
                const fps = 1000 / delta;
                frames.push(fps);
            }

            lastTime = currentTime;
            requestAnimationFrame(measureFrame);
        }

        requestAnimationFrame(measureFrame);
    });
})();
`, int(duration.Milliseconds()))

	var result string
	err := chromedp.Run(mc.ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return nil, fmt.Errorf("failed to collect FPS metrics: %w", err)
	}

	var fpsMetrics FPSMetrics
	if err := json.Unmarshal([]byte(result), &fpsMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse FPS metrics: %w", err)
	}

	return &fpsMetrics, nil
}

// CollectLoadTime gathers page load timing information
func (mc *MetricsCollector) CollectLoadTime() (*LoadTimeMetrics, error) {
	script := `
(function() {
    const timing = performance.timing;
    const navigation = timing;

    // Calculate timing metrics
    const metrics = {
        dnsLookup: navigation.domainLookupEnd - navigation.domainLookupStart,
        tcpConnection: navigation.connectEnd - navigation.connectStart,
        serverResponse: navigation.responseStart - navigation.requestStart,
        pageDownload: navigation.responseEnd - navigation.responseStart,
        domContentLoaded: navigation.domContentLoadedEventEnd - navigation.navigationStart,
        windowLoad: navigation.loadEventEnd - navigation.navigationStart,
        totalLoadTime: navigation.loadEventEnd - navigation.navigationStart
    };

    // Get resource timing information
    const resources = performance.getEntriesByType('resource');
    let largestResource = { size: 0, url: '' };

    resources.forEach(resource => {
        if (resource.transferSize > largestResource.size) {
            largestResource = {
                size: resource.transferSize,
                url: resource.name
            };
        }
    });

    metrics.resourceCount = resources.length;
    metrics.largestResourceSize = largestResource.size;
    metrics.largestResourceURL = largestResource.url;

    // Get paint timing
    const paintTiming = performance.getEntriesByType('paint');
    metrics.firstPaint = 0;
    metrics.firstContentfulPaint = 0;

    paintTiming.forEach(entry => {
        if (entry.name === 'first-paint') {
            metrics.firstPaint = entry.startTime;
        } else if (entry.name === 'first-contentful-paint') {
            metrics.firstContentfulPaint = entry.startTime;
        }
    });

    return JSON.stringify(metrics);
})();
`

	var result string
	err := chromedp.Run(mc.ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return nil, fmt.Errorf("failed to collect load time metrics: %w", err)
	}

	// Parse the result
	var rawMetrics struct {
		DNSLookup            int64   `json:"dnsLookup"`
		TCPConnection        int64   `json:"tcpConnection"`
		ServerResponse       int64   `json:"serverResponse"`
		PageDownload         int64   `json:"pageDownload"`
		DOMContentLoaded     int64   `json:"domContentLoaded"`
		WindowLoad           int64   `json:"windowLoad"`
		TotalLoadTime        int64   `json:"totalLoadTime"`
		ResourceCount        int     `json:"resourceCount"`
		LargestResourceSize  int64   `json:"largestResourceSize"`
		LargestResourceURL   string  `json:"largestResourceURL"`
		FirstPaint           float64 `json:"firstPaint"`
		FirstContentfulPaint float64 `json:"firstContentfulPaint"`
	}

	if err := json.Unmarshal([]byte(result), &rawMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse load time metrics: %w", err)
	}

	loadTimeMetrics := &LoadTimeMetrics{
		DNSLookup:            rawMetrics.DNSLookup,
		TCPConnection:        rawMetrics.TCPConnection,
		ServerResponse:       rawMetrics.ServerResponse,
		PageDownload:         rawMetrics.PageDownload,
		DOMContentLoaded:     rawMetrics.DOMContentLoaded,
		WindowLoad:           rawMetrics.WindowLoad,
		TotalLoadTime:        rawMetrics.TotalLoadTime,
		ResourceCount:        rawMetrics.ResourceCount,
		LargestResourceSize:  rawMetrics.LargestResourceSize,
		LargestResourceURL:   rawMetrics.LargestResourceURL,
		FirstPaint:           rawMetrics.FirstPaint,
		FirstContentfulPaint: rawMetrics.FirstContentfulPaint,
	}

	return loadTimeMetrics, nil
}

// CollectAccessibility performs automated accessibility checks
func (mc *MetricsCollector) CollectAccessibility() (*AccessibilityReport, error) {
	// First, inject axe-core library
	injectScript := `
(function() {
    return new Promise(function(resolve, reject) {
        // Check if axe is already loaded
        if (window.axe) {
            resolve('already-loaded');
            return;
        }

        // Load axe-core from CDN
        const script = document.createElement('script');
        script.src = 'https://cdnjs.cloudflare.com/ajax/libs/axe-core/4.8.2/axe.min.js';
        script.onload = function() {
            resolve('loaded');
        };
        script.onerror = function() {
            reject('failed-to-load');
        };
        document.head.appendChild(script);
    });
})();
`

	var injectResult string
	err := chromedp.Run(mc.ctx, chromedp.Evaluate(injectScript, &injectResult))
	if err != nil {
		return nil, fmt.Errorf("failed to inject axe-core: %w", err)
	}

	// Wait a moment for axe to fully initialize
	time.Sleep(500 * time.Millisecond)

	// Run axe accessibility checks
	checkScript := `
(function() {
    return new Promise(function(resolve, reject) {
        if (!window.axe) {
            reject('axe-not-loaded');
            return;
        }

        axe.run(document, {
            runOnly: {
                type: 'tag',
                values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']
            }
        }).then(function(results) {
            const report = {
                violationCount: results.violations.length,
                warningCount: results.incomplete.length,
                passCount: results.passes.length,
                violations: [],
                warnings: []
            };

            // Process violations
            results.violations.forEach(function(violation) {
                report.violations.push({
                    rule: violation.id,
                    impact: violation.impact || 'unknown',
                    description: violation.description,
                    helpURL: violation.helpUrl,
                    elements: violation.nodes.map(function(node) {
                        return node.target.join(' ');
                    }),
                    count: violation.nodes.length
                });
            });

            // Process warnings (incomplete checks)
            results.incomplete.forEach(function(warning) {
                report.warnings.push({
                    rule: warning.id,
                    impact: warning.impact || 'unknown',
                    description: warning.description,
                    helpURL: warning.helpUrl,
                    elements: warning.nodes.map(function(node) {
                        return node.target.join(' ');
                    }),
                    count: warning.nodes.length
                });
            });

            resolve(JSON.stringify(report));
        }).catch(function(error) {
            reject('axe-run-failed: ' + error.message);
        });
    });
})();
`

	var checkResult string
	err = chromedp.Run(mc.ctx, chromedp.Evaluate(checkScript, &checkResult))
	if err != nil {
		return nil, fmt.Errorf("failed to run accessibility checks: %w", err)
	}

	// Parse results
	var rawReport struct {
		ViolationCount int                      `json:"violationCount"`
		WarningCount   int                      `json:"warningCount"`
		PassCount      int                      `json:"passCount"`
		Violations     []AccessibilityViolation `json:"violations"`
		Warnings       []AccessibilityViolation `json:"warnings"`
	}

	if err := json.Unmarshal([]byte(checkResult), &rawReport); err != nil {
		return nil, fmt.Errorf("failed to parse accessibility report: %w", err)
	}

	// Calculate score (100 = perfect, deduct points for violations)
	score := 100
	for _, v := range rawReport.Violations {
		switch v.Impact {
		case "critical":
			score -= 15
		case "serious":
			score -= 10
		case "moderate":
			score -= 5
		case "minor":
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}

	// Generate summary
	summary := fmt.Sprintf("Found %d violations and %d warnings. ",
		rawReport.ViolationCount, rawReport.WarningCount)

	if rawReport.ViolationCount == 0 {
		summary += "Page meets WCAG 2.1 AA standards."
	} else if score >= 80 {
		summary += "Minor accessibility issues detected."
	} else if score >= 60 {
		summary += "Moderate accessibility issues detected."
	} else {
		summary += "Significant accessibility issues detected."
	}

	report := &AccessibilityReport{
		Score:          score,
		ViolationCount: rawReport.ViolationCount,
		WarningCount:   rawReport.WarningCount,
		PassCount:      rawReport.PassCount,
		Violations:     rawReport.Violations,
		Warnings:       rawReport.Warnings,
		Summary:        summary,
	}

	return report, nil
}
