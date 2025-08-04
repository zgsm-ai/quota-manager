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

// 固定重试参数
const (
	MaxRetries        = 3                      // 最大重试次数
	InitialDelay      = 100 * time.Millisecond // 初始延迟
	BackoffMultiplier = 2.0                    // 退避倍数
	MaxDelay          = 5 * time.Second        // 最大延迟
)

// HTTPError 包装 HTTP 状态码错误
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error %d: %s", e.StatusCode, e.Message)
}

// RetryableError 判断错误是否可重试
func RetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 网络错误通常可重试
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return true
		}
		// 其他网络错误
		return true
	}

	// 检查是否为 HTTP 错误
	if httpErr, ok := err.(*HTTPError); ok {
		// 5xx 服务器错误可重试
		if httpErr.StatusCode >= 500 && httpErr.StatusCode < 600 {
			return true
		}
		// 429 Too Many Requests 可重试
		if httpErr.StatusCode == http.StatusTooManyRequests {
			return true
		}
	}

	return false
}

// WithRetry 为函数添加重试机制
func WithRetry[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算退避时间
			delay := calculateBackoff(attempt, InitialDelay, BackoffMultiplier, MaxDelay)

			// 添加抖动
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

		// 检查错误是否可重试
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

// calculateBackoff 计算退避时间
func calculateBackoff(attempt int, initialDelay time.Duration, multiplier float64, maxDelay time.Duration) time.Duration {
	delay := float64(initialDelay) * math.Pow(multiplier, float64(attempt-1))

	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	return time.Duration(delay)
}
