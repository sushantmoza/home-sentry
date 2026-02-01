package network

import (
	"time"
)

// RetryConfig holds configuration for retry operations
type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		Delay:       500 * time.Millisecond,
		Multiplier:  1.5,
	}
}

// Retry executes the given function with retry logic
func Retry(config RetryConfig, operation func() error) error {
	var err error
	delay := config.Delay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}

		if attempt < config.MaxAttempts {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * config.Multiplier)
		}
	}

	return err
}

// RetryWithResult executes the given function with retry logic and returns a result
func RetryWithResult[T any](config RetryConfig, operation func() (T, error)) (T, error) {
	var result T
	var err error
	delay := config.Delay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		result, err = operation()
		if err == nil {
			return result, nil
		}

		if attempt < config.MaxAttempts {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * config.Multiplier)
		}
	}

	return result, err
}
