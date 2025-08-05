package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"

	"quota-manager/pkg/logger"

	"go.uber.org/zap"
)

// Fixed retry parameters
const (
	MaxRetries        = 3                      // Maximum retry attempts
	InitialDelay      = 100 * time.Millisecond // Initial delay
	BackoffMultiplier = 2.0                    // Backoff multiplier
	MaxDelay          = 5 * time.Second        // Maximum delay
)

// HTTPError wraps HTTP status code errors
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error %d: %s", e.StatusCode, e.Message)
}

// RetryableError determines if an error is retryable
func RetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are usually retryable
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return true
		}
		// Other network errors
		return true
	}

	// Check if it's an HTTP error
	if httpErr, ok := err.(*HTTPError); ok {
		// 5xx server errors are retryable
		if httpErr.StatusCode >= 500 && httpErr.StatusCode < 600 {
			return true
		}
		// 429 Too Many Requests is retryable
		if httpErr.StatusCode == http.StatusTooManyRequests {
			return true
		}
	}

	return false
}

// WithRetry adds retry mechanism to function
func WithRetry[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff time
			delay := calculateBackoff(attempt, InitialDelay, BackoffMultiplier, MaxDelay)

			// Add jitter
			jitter := time.Duration(rand.Int63n(int64(delay / 4)))
			delay += jitter

			logger.Logger.Info("Retry attempt",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", MaxRetries),
				zap.Duration("delay", delay))

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}

		result, err := operation()
		if err == nil {
			if attempt > 0 {
				logger.Logger.Info("Operation succeeded",
					zap.Int("attempts", attempt))
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !RetryableError(err) {
			logger.Logger.Warn("Non-retryable error occurred",
				zap.Error(err))
			return zero, err
		}

		if attempt < MaxRetries {
			logger.Logger.Warn("Attempt failed, will retry",
				zap.Int("attempt", attempt+1),
				zap.Error(err))
		} else {
			logger.Logger.Error("All retry attempts failed",
				zap.Int("total_attempts", MaxRetries+1),
				zap.Error(err))
		}
	}

	return zero, lastErr
}

// calculateBackoff calculates backoff time
func calculateBackoff(attempt int, initialDelay time.Duration, multiplier float64, maxDelay time.Duration) time.Duration {
	delay := float64(initialDelay) * math.Pow(multiplier, float64(attempt-1))

	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	return time.Duration(delay)
}
