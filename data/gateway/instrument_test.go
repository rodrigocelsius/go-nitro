package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

func TestInferOperation(t *testing.T) {
	cases := map[string]string{
		"SELECT 1":                             "select",
		"  insert into t":                      "insert",
		"UPDATE t SET x=1":                     "update",
		"DELETE FROM t":                        "delete",
		"WITH x AS (SELECT 1) SELECT * FROM x": "select",
	}
	for sql, want := range cases {
		if got := gateway.InferOperation(sql); got != want {
			t.Fatalf("%q: got %s want %s", sql, got, want)
		}
	}
}

func TestInstrument(t *testing.T) {
	exec := &stubExec{}
	gw := gateway.New(exec, gateway.DisableObservability())
	var called bool
	err := gw.Instrument(context.Background(), "select", "SELECT 1", nil, func() (int, error) {
		called = true
		return 1, nil
	})
	if err != nil || !called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	err = gw.Instrument(context.Background(), "select", "SELECT 1", nil, func() (int, error) {
		return 0, errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

type stubExec struct{}

func (stubExec) Query(context.Context, string, []any) (gateway.Rows, error) {
	return nil, errors.New("unused")
}
func (stubExec) QueryRow(context.Context, string, []any) gateway.Row { return nil }
func (stubExec) Exec(context.Context, string, []any) (gateway.Result, error) {
	return nil, errors.New("unused")
}
