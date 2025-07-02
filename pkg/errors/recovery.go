package errors

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/pkg/logger"
)

// -----
// Recovery Functions
// -----

// RecoverWithContext handles panics and converts them to errors
func RecoverWithContext(_ context.Context, operation string) error {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		logger.Error("panic recovered",
			"operation", operation,
			"panic", r,
			"stack", stack,
		)

		// Convert panic to error
		var err error
		switch v := r.(type) {
		case error:
			err = v
		case string:
			err = errors.New(v)
		default:
			err = fmt.Errorf("panic: %v", v)
		}

		return core.NewError(err, "PANIC_RECOVERED", map[string]any{
			"operation": operation,
			"panic":     fmt.Sprintf("%v", r),
		})
	}
	return nil
}

// WithRecover executes a function with panic recovery
func WithRecover(operation string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("panic recovered",
				"operation", operation,
				"panic", r,
				"stack", stack,
			)

			// Convert panic to error
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("panic: %v", v)
			}

			err = core.NewError(err, "PANIC_RECOVERED", map[string]any{
				"operation": operation,
				"panic":     fmt.Sprintf("%v", r),
			})
		}
	}()

	return fn()
}

// -----
// Retry Mechanisms using retry-go
// -----

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     uint
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RetryableErrors []string // Error codes that should trigger retry
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: []string{
			"NEO4J_CONNECTION_ERROR",
			"TIMEOUT_ERROR",
			"TEMPORARY_ERROR",
		},
	}
}

// WithRetry executes a function with retry logic using retry-go
func WithRetry(ctx context.Context, operation string, config *RetryConfig, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	// Create retry options
	opts := []retry.Option{
		retry.Attempts(config.MaxAttempts),
		retry.Delay(config.InitialDelay),
		retry.MaxDelay(config.MaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			logger.Warn("operation failed, retrying",
				"operation", operation,
				"attempt", n+1,
				"max_attempts", config.MaxAttempts,
				"error", err,
			)
		}),
		// Only retry if the error is retryable
		retry.RetryIf(func(err error) bool {
			return isRetryable(err, config.RetryableErrors)
		}),
	}

	err := retry.Do(fn, opts...)
	if err != nil {
		// Check if it's a max retries error
		if retry.IsRecoverable(err) {
			return core.NewError(err, "MAX_RETRIES_EXCEEDED", map[string]any{
				"operation": operation,
				"attempts":  config.MaxAttempts,
			})
		}
		return err
	}

	return nil
}

// WithRetryTyped executes a function with retry logic and returns a typed result
func WithRetryTyped[T any](
	ctx context.Context,
	operation string,
	config *RetryConfig,
	fn func() (T, error),
) (T, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var result T

	// Create retry options
	opts := []retry.Option{
		retry.Attempts(config.MaxAttempts),
		retry.Delay(config.InitialDelay),
		retry.MaxDelay(config.MaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			logger.Warn("operation failed, retrying",
				"operation", operation,
				"attempt", n+1,
				"max_attempts", config.MaxAttempts,
				"error", err,
			)
		}),
		// Only retry if the error is retryable
		retry.RetryIf(func(err error) bool {
			return isRetryable(err, config.RetryableErrors)
		}),
	}

	err := retry.Do(func() error {
		var retryErr error
		result, retryErr = fn()
		return retryErr
	}, opts...)

	if err != nil {
		// Check if it's a max retries error
		if retry.IsRecoverable(err) {
			return result, core.NewError(err, "MAX_RETRIES_EXCEEDED", map[string]any{
				"operation": operation,
				"attempts":  config.MaxAttempts,
			})
		}
		return result, err
	}

	return result, nil
}

// isRetryable checks if an error should trigger a retry
func isRetryable(err error, retryableCodes []string) bool {
	if err == nil {
		return false
	}

	// Check if it's a core.Error with a retryable code
	if coreErr, ok := err.(*core.Error); ok {
		for _, code := range retryableCodes {
			if coreErr.Code == core.ErrorCode(code) {
				return true
			}
		}
	}

	// Check for wrapped core.Error
	var coreErr *core.Error
	if unwrapErr := err; unwrapErr != nil {
		for {
			if ce, ok := unwrapErr.(*core.Error); ok {
				coreErr = ce
				break
			}
			unwrapErr = fmt.Errorf("%w", unwrapErr)
			if unwrapErr == err {
				break
			}
			err = unwrapErr
		}
	}

	if coreErr != nil {
		for _, code := range retryableCodes {
			if coreErr.Code == core.ErrorCode(code) {
				return true
			}
		}
	}

	return false
}

// -----
// Graceful Degradation
// -----

// GracefulDegradeConfig configures graceful degradation behavior
type GracefulDegradeConfig struct {
	LogWarning bool
}

// WithGracefulDegrade executes a function and returns a default value on error
func WithGracefulDegrade[T any](operation string, config *GracefulDegradeConfig, defaultVal T, fn func() (T, error)) T {
	result, err := fn()
	if err != nil {
		if config != nil && config.LogWarning {
			logger.Warn("operation degraded gracefully",
				"operation", operation,
				"error", err,
				"default_value", defaultVal,
			)
		}
		return defaultVal
	}
	return result
}

// -----
// Circuit Breaker Pattern
// -----

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold uint          // Number of failures before opening circuit
	SuccessThreshold uint          // Number of successes before closing circuit
	Timeout          time.Duration // Time to wait before attempting to close circuit
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}
