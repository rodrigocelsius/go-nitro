package fasthttp

import (
	"context"
	"testing"
	"unsafe"
)

// The facade converts *domain.Ctx <-> *FastCtx with unsafe.Pointer, which is
// only valid while the embedded domain.Ctx is the FIRST field of FastCtx.
// If someone reorders the struct this test fails instead of corrupting memory.
func TestFastCtx_DomainCtxIsFirstField(t *testing.T) {
	var c FastCtx
	if off := unsafe.Offsetof(c.Ctx); off != 0 {
		t.Fatalf("FastCtx.Ctx must stay at offset 0 (facade AsCtx uses unsafe.Pointer), got %d", off)
	}
}

type ctxKey struct{}

func TestDetach_SurvivesCancellationAndKeepsValues(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "v"))
	c := &FastCtx{reqCtx: parent}
	d := c.Detach()
	cancel()
	if d.Err() != nil {
		t.Fatalf("detached ctx must not be cancelled with the request: %v", d.Err())
	}
	if d.Value(ctxKey{}) != "v" {
		t.Fatal("detached ctx must keep request-context values")
	}
}
