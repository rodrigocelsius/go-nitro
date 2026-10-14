package pgx

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// retryableBackoff returns the wait before attempt n (1-based): base * 2^(n-1),
// capped at 10×base. Deterministic (no jitter) so tests stay fast.
func retryableBackoff(base time.Duration, attempt int) time.Duration {
	d := base
	for i := 1; i < attempt && d < 10*base; i++ {
		d *= 2
	}
	return d
}

// IsRetryable reports whether err is a transient serialization failure or
// deadlock that a transaction retry can resolve (SQLSTATE 40001 / 40P01).
func IsRetryable(err error) bool {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		switch pgerr.Code {
		case "40001", // serialization_failure
			"40P01": // deadlock_detected
			return true
		}
	}
	return false
}

// WithRetry runs fn, retrying when it fails with a retryable error
// (serialization failure / deadlock). fn must be idempotent or wrap its whole
// transaction — the standard usage re-runs the entire Tx:
//
//	err := pgx.WithRetry(ctx, 3, 100*time.Millisecond, func(ctx context.Context) error {
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
