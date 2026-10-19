package main

import (
	"runtime/debug"
	"testing"
)

func TestModuleVersion(t *testing.T) {
	stamped := func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v2.4.1"}}, true
	}
	if got := moduleVersion(stamped); got != "2.4.1" {
		t.Fatalf("stamped = %q", got)
	}
	devel := func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	}
	if got := moduleVersion(devel); got != devVersion {
		t.Fatalf("devel = %q", got)
	}
}
