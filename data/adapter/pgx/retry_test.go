package pgx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func pgErr(code string) error {
	return &pgconn.PgError{Code: code, Message: "boom"}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(pgErr("40001")) || !IsRetryable(pgErr("40P01")) {
		t.Fatal("40001/40P01 must be retryable")
	}
	if IsRetryable(pgErr("23505")) {
		t.Fatal("unique violation must not be retryable")
	}
	if IsRetryable(errors.New("plain")) {
		t.Fatal("plain errors must not be retryable")
	}
}

func TestWithRetry_RetriesThenSucceeds(t *testing.T) {
	attempts := 0
	err := WithRetry(context.Background(), 3, time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return pgErr("40001")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestWithRetry_NonRetryableFailsFast(t *testing.T) {
	attempts := 0
	want := errors.New("fatal")
	err := WithRetry(context.Background(), 5, time.Millisecond, func(ctx context.Context) error {
		attempts++
		return want
	})
	if !errors.Is(err, want) || attempts != 1 {
		t.Fatalf("non-retryable should fail fast: err=%v attempts=%d", err, attempts)
	}
}

func TestWithRetry_ExhaustsAttempts(t *testing.T) {
	attempts := 0
	err := WithRetry(context.Background(), 4, time.Millisecond, func(ctx context.Context) error {
		attempts++
		return pgErr("40P01")
	})
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || attempts != 4 {
		t.Fatalf("expected exhaustion at 4 attempts, got %d (%v)", attempts, err)
	}
}

func TestWithRetry_ContextCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go func() {
		defer cancel()
		<-started
	}()
	err := WithRetry(ctx, 5, 50*time.Millisecond, func(ctx context.Context) error {
		close(started)
		return pgErr("40001") // first failure → backoff → ctx cancelled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
