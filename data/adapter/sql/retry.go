package sqladapter

import (
	"context"
	"errors"
	"time"
)

// mysqlRetryable is set by driver_mysql.go (absent with -tags nitro_nomysql).
var mysqlRetryable = func(error) bool { return false }

// IsRetryable reports whether err is a transient serialization failure or
// deadlock that a transaction retry can resolve:
//   - SQLSTATE 40001 (serialization_failure) / 40P01 (deadlock_detected) — PostgreSQL
//   - MySQL/MariaDB error 1213 (deadlock) / 1205 (lock wait timeout)
func IsRetryable(err error) bool {
	if mysqlRetryable(err) {
		return true
	}
	var state sqlstate
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "40001", "40P01":
			return true
		}
	}
	return false
}

type sqlstate interface{ SQLState() string }

// retryableBackoff returns the wait before attempt n (1-based): base * 2^(n-1),
// capped at 10×base.
func retryableBackoff(base time.Duration, attempt int) time.Duration {
	d := base
	for i := 1; i < attempt && d < 10*base; i++ {
		d *= 2
	}
	return d
}

// WithRetry runs fn, retrying when it fails with a retryable error
// (serialization failure / deadlock). fn must re-run the whole transaction:
//
//	err := sqladapter.WithRetry(ctx, 3, 100*time.Millisecond, func(ctx context.Context) error {
//	    return db.Tx(ctx, transferMoney)
//	})
func WithRetry(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func(ctx context.Context) error) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsRetryable(err) || attempt == maxAttempts {
			return err
		}
		timer := time.NewTimer(retryableBackoff(baseDelay, attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}
