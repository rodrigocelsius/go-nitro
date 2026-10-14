package main

import (
	"math"
	"testing"
)

func TestParseCoverFunc(t *testing.T) {
	in := `
github.com/acme/app/internal/config/config.go:30:		Load		100.0%
github.com/acme/app/cmd/api/main.go:17:			main		0.0%
github.com/acme/app/internal/handlers/routes.go:11:	Register	0.0%
github.com/acme/app/internal/handlers/routes.go:20:	MountAPI	100.0%
total:							(statements)	23.8%
`
	rep, err := parseCoverFunc(in, []string{"cmd/"})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(rep.Total-23.8) > 0.01 {
		t.Fatalf("total=%v", rep.Total)
	}
	if !rep.Filtered {
		t.Fatal("expected filtered")
	}
	if rep.Excluded != 1 {
		t.Fatalf("excluded=%d want 1 (cmd/api/main.go)", rep.Excluded)
	}
	// remaining: Load 100, Register 0, MountAPI 100 → avg 66.666...
	want := (100.0 + 0.0 + 100.0) / 3
	if math.Abs(rep.FilteredPct-want) > 0.01 {
		t.Fatalf("filtered=%v want %v", rep.FilteredPct, want)
	}
}

func TestCoverExcluded(t *testing.T) {
	if !coverExcluded("github.com/x/cmd/api/main.go", []string{"cmd/", "main.go"}) {
		t.Fatal("expected exclude")
	}
	if coverExcluded("github.com/x/internal/handlers/routes.go", []string{"cmd/"}) {
		t.Fatal("should not exclude handlers")
	}
}

func TestNormalizeTestArgs(t *testing.T) {
	got := normalizeTestArgs([]string{"--verbose", "--cover-min", "60", "./internal/..."})
	want := []string{"-v", "-min", "60", "./internal/..."}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}
