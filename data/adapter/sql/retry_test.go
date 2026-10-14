package sqladapter

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type stateErr struct{ code string }

func (e stateErr) Error() string    { return "sql error " + e.code }
func (e stateErr) SQLState() string { return e.code }

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"pg serialization", stateErr{"40001"}, true},
		{"pg deadlock", stateErr{"40P01"}, true},
		{"pg unique", stateErr{"23505"}, false},
		{"mysql deadlock", &mysql.MySQLError{Number: 1213, Message: "deadlock"}, true},
		{"mysql lock timeout", &mysql.MySQLError{Number: 1205, Message: "timeout"}, true},
		{"mysql dup key", &mysql.MySQLError{Number: 1062, Message: "duplicate"}, false},
		{"plain", errors.New("nope"), false},
		{"driver.ErrBadConn", driver.ErrBadConn, false},
	}
	for _, tc := range cases {
		if got := IsRetryable(tc.err); got != tc.want {
			t.Errorf("%s: IsRetryable=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestWithRetry_RetriesMySQLDeadlock(t *testing.T) {
	attempts := 0
	err := WithRetry(context.Background(), 3, time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return &mysql.MySQLError{Number: 1213, Message: "deadlock found"}
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}
