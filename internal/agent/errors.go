package agent

import (
	"context"
	"fmt"
	"time"
)

// ErrorCategory represents the type of error
type ErrorCategory string

const (
	// ErrorCategoryBrowser for browser-related errors
	ErrorCategoryBrowser ErrorCategory = "browser"
	// ErrorCategoryNetwork for network/connectivity errors
	ErrorCategoryNetwork ErrorCategory = "network"
	// ErrorCategoryTimeout for timeout errors
	ErrorCategoryTimeout ErrorCategory = "timeout"
	// ErrorCategoryLLM for LLM API errors
	ErrorCategoryLLM ErrorCategory = "llm"
	// ErrorCategoryStorage for S3/storage errors
	ErrorCategoryStorage ErrorCategory = "storage"
	// ErrorCategoryUnknown for uncategorized errors
	ErrorCategoryUnknown ErrorCategory = "unknown"
)

// CategorizedError wraps an error with category and retry info
type CategorizedError struct {
	Category  ErrorCategory
	Original  error
	Retryable bool
	Message   string
}

// Error implements the error interface
func (e *CategorizedError) Error() string {
	return fmt.Sprintf("[%s] %s: %v", e.Category, e.Message, e.Original)
}

// Unwrap implements error unwrapping
func (e *CategorizedError) Unwrap() error {
	return e.Original
}

// NewBrowserError creates a browser-related error
func NewBrowserError(message string, err error) *CategorizedError {
	return &CategorizedError{
		Category:  ErrorCategoryBrowser,
		Original:  err,
		Retryable: true,
		Message:   message,
	}
}

// NewNetworkError creates a network-related error
func NewNetworkError(message string, err error) *CategorizedError {
	return &CategorizedError{
		Category:  ErrorCategoryNetwork,
		Original:  err,
		Retryable: true,
		Message:   message,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, err error) *CategorizedError {
	return &CategorizedError{
		Category:  ErrorCategoryTimeout,
		Original:  err,
		Retryable: true,
		Message:   message,
	}
}

// NewLLMError creates an LLM API error
func NewLLMError(message string, err error) *CategorizedError {
	return &CategorizedError{
		Category:  ErrorCategoryLLM,
		Original:  err,
		Retryable: true,
		Message:   message,
	}
}

// NewStorageError creates a storage error
func NewStorageError(message string, err error) *CategorizedError {
	return &CategorizedError{
		Category:  ErrorCategoryStorage,
		Original:  err,
		Retryable: true,
		Message:   message,
	}
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []ErrorCategory
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []ErrorCategory{
			ErrorCategoryNetwork,
			ErrorCategoryTimeout,
			ErrorCategoryLLM,
		},
	}
}

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !shouldRetry(err, config) {
			return err
		}

		// Check if we have more attempts
		if attempt < config.MaxAttempts-1 {
			// Calculate delay with exponential backoff
			delay := calculateDelay(attempt, config)

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// shouldRetry determines if an error is retryable
func shouldRetry(err error, config RetryConfig) bool {
	// Check if it's a categorized error
	catErr, ok := err.(*CategorizedError)
	if !ok {
		// Unknown errors are not retryable by default
		return false
	}

	if !catErr.Retryable {
		return false
	}

	// Check if category is in retryable list
	for _, category := range config.RetryableErrors {
		if catErr.Category == category {
			return true
		}
	}

	return false
}

// calculateDelay calculates retry delay with exponential backoff
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	delay := float64(config.InitialDelay)

	// Apply exponential backoff
	for i := 0; i < attempt; i++ {
		delay *= config.BackoffFactor
	}

	// Cap at max delay
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}

// WithRetry wraps a function with default retry logic
func WithRetry(ctx context.Context, fn func() error) error {
	return Retry(ctx, DefaultRetryConfig(), fn)
}
