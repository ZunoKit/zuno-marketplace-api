package resilience

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig defines retry behavior configuration
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFraction  float64
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    3,
		InitialDelay:   1 * time.Second,
		MaxDelay:       30 * time.Second,
		BackoffFactor:  2.0,
		JitterFraction: 0.1,
		RetryableErrors: func(err error) bool {
			// By default, retry all errors
			return true
		},
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context) error

// RetryWithConfig executes a function with retry logic based on the provided configuration
func RetryWithConfig(ctx context.Context, config *RetryConfig, fn RetryableFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't retry if this was the last attempt
		if attempt >= config.MaxAttempts {
			break
		}

		// Calculate next delay with exponential backoff
		delay = calculateBackoff(delay, config)

		// Log retry attempt
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry attempt
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// Retry executes a function with default retry configuration
func Retry(ctx context.Context, fn RetryableFunc) error {
	return RetryWithConfig(ctx, DefaultRetryConfig(), fn)
}

// RetryWithExponentialBackoff retries with exponential backoff
func RetryWithExponentialBackoff(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn RetryableFunc) error {
	config := &RetryConfig{
		MaxAttempts:    maxAttempts,
		InitialDelay:   initialDelay,
		MaxDelay:       initialDelay * time.Duration(math.Pow(2, float64(maxAttempts))),
		BackoffFactor:  2.0,
		JitterFraction: 0.1,
		RetryableErrors: func(err error) bool {
			return true
		},
	}
	return RetryWithConfig(ctx, config, fn)
}

// calculateBackoff calculates the next delay with exponential backoff and jitter
func calculateBackoff(currentDelay time.Duration, config *RetryConfig) time.Duration {
	// Apply exponential backoff
	nextDelay := time.Duration(float64(currentDelay) * config.BackoffFactor)

	// Cap at maximum delay
	if nextDelay > config.MaxDelay {
		nextDelay = config.MaxDelay
	}

	// Add jitter to prevent thundering herd
	if config.JitterFraction > 0 {
		jitter := time.Duration(rand.Float64() * config.JitterFraction * float64(nextDelay))
		nextDelay = nextDelay + jitter
	}

	return nextDelay
}

// RetryWithCustomBackoff allows custom backoff strategies
type BackoffStrategy func(attempt int) time.Duration

// LinearBackoff returns a linear backoff strategy
func LinearBackoff(baseDelay time.Duration) BackoffStrategy {
	return func(attempt int) time.Duration {
		return baseDelay * time.Duration(attempt)
	}
}

// ExponentialBackoff returns an exponential backoff strategy
func ExponentialBackoff(baseDelay time.Duration, factor float64) BackoffStrategy {
	return func(attempt int) time.Duration {
		return time.Duration(float64(baseDelay) * math.Pow(factor, float64(attempt-1)))
	}
}

// FibonacciBackoff returns a Fibonacci sequence backoff strategy
func FibonacciBackoff(baseDelay time.Duration) BackoffStrategy {
	return func(attempt int) time.Duration {
		a, b := 0, 1
		for i := 0; i < attempt; i++ {
			a, b = b, a+b
		}
		return baseDelay * time.Duration(a)
	}
}

// RetryWithBackoffStrategy retries with a custom backoff strategy
func RetryWithBackoffStrategy(ctx context.Context, maxAttempts int, strategy BackoffStrategy, fn RetryableFunc) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check context
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Execute function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't delay after last attempt
		if attempt >= maxAttempts {
			break
		}

		// Calculate delay using strategy
		delay := strategy(attempt)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", maxAttempts, lastErr)
}
