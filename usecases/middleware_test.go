package usecases

import (
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestCompileHandler_NoMiddleware(t *testing.T) {
	called := false
	handler := func(c *domain.Ctx) error {
		called = true
		return nil
	}

	compiled := CompileHandler(handler, nil, nil, nil)
	if compiled == nil {
		t.Fatal("expected non-nil handler")
	}

	err := compiled(&domain.Ctx{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestCompileHandler_GlobalPre(t *testing.T) {
	order := []string{}
	handler := func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	}
	pre1 := func(c *domain.Ctx) error {
		order = append(order, "pre1")
		return nil
	}
	pre2 := func(c *domain.Ctx) error {
		order = append(order, "pre2")
		return nil
	}

	compiled := CompileHandler(handler, []domain.Handler{pre1, pre2}, nil, nil)
	compiled(&domain.Ctx{})

	if len(order) != 3 || order[0] != "pre1" || order[1] != "pre2" || order[2] != "handler" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestCompileHandler_GlobalPost(t *testing.T) {
	order := []string{}
	handler := func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	}
	post1 := func(c *domain.Ctx) error {
		order = append(order, "post1")
		return nil
	}

	compiled := CompileHandler(handler, nil, []domain.Handler{post1}, nil)
	compiled(&domain.Ctx{})

	if len(order) != 2 || order[0] != "handler" || order[1] != "post1" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestCompileHandler_RouteMiddleware(t *testing.T) {
	order := []string{}
	handler := func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	}
	globalPre := func(c *domain.Ctx) error {
		order = append(order, "globalPre")
		return nil
	}
	routeMw := func(c *domain.Ctx) error {
		order = append(order, "routeMw")
		return nil
	}

	compiled := CompileHandler(handler, []domain.Handler{globalPre}, nil, []domain.Handler{routeMw})
	compiled(&domain.Ctx{})

	expected := []string{"globalPre", "routeMw", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("unexpected order length: %v", order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d] = %q, got %q", i, v, order[i])
		}
	}
}

func TestCompileHandler_ErrorStillRunsPost(t *testing.T) {
	preErr := errors.New("pre error")
	handler := func(c *domain.Ctx) error {
		t.Fatal("handler should not be called")
		return nil
	}
	pre := func(c *domain.Ctx) error {
		return preErr
	}
	postCalled := false
	post := func(c *domain.Ctx) error {
		postCalled = true
		return nil
	}

	compiled := CompileHandler(handler, []domain.Handler{pre}, []domain.Handler{post}, nil)
	err := compiled(&domain.Ctx{})
	if err != preErr {
		t.Fatalf("expected preErr, got %v", err)
	}
	if !postCalled {
		t.Fatal("post should run even when pre fails (access log / span end)")
	}
}

func TestCompileHandler_HandlerErrorRunsPost(t *testing.T) {
	handlerErr := errors.New("boom")
	postCalled := false
	compiled := CompileHandler(
		func(c *domain.Ctx) error { return handlerErr },
		nil,
		[]domain.Handler{func(c *domain.Ctx) error { postCalled = true; return nil }},
		nil,
	)
	err := compiled(&domain.Ctx{})
	if err != handlerErr {
		t.Fatalf("expected handlerErr, got %v", err)
	}
	if !postCalled {
		t.Fatal("post should run after handler error")
	}
}
