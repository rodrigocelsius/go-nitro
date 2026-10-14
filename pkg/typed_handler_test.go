package nitro

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type thGet struct {
	ID int64 `validate:"required"` // bound from ":id" by convention
}
type thCreate struct {
	Name string `json:"name" validate:"required"`
}
type thUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func thApp() *App {
	app := New(Config{DisableStartupMessage: true})
	app.Get("/users/:id", Handle(func(ctx context.Context, in *thGet) (*thUser, error) {
		if in.ID == 404 {
			return nil, NotFound("no such user")
		}
		return &thUser{ID: in.ID, Name: "Ana"}, nil
	}))
	app.Post("/users", Handle(func(ctx context.Context, in *thCreate) (thUser, error) {
		return thUser{ID: 1, Name: in.Name}, nil
	}, 201))
	app.Delete("/users/:id", Handle(func(ctx context.Context, in *thGet) (Empty, error) { return Empty{}, nil }))
	app.Get("/boom", Handle(func(ctx context.Context, _ *Empty) (*thUser, error) { return nil, errors.New("x") }))
	return app
}

func TestHandle_PathParamAndJSON(t *testing.T) {
	app := thApp()
	resp, err := app.Test("GET", "/users/7", nil)
	if err != nil || resp.StatusCode() != 200 || !strings.Contains(string(resp.Body()), `"id":7`) {
		t.Fatalf("get: %v %d %s", err, resp.StatusCode(), resp.Body())
	}
	resp, _ = app.Test("GET", "/users/abc", nil)
	if resp.StatusCode() != 400 {
		t.Fatalf("bad param: %d %s", resp.StatusCode(), resp.Body())
	}
	resp, _ = app.Test("GET", "/users/404", nil)
	if resp.StatusCode() != 404 {
		t.Fatalf("not found: %d", resp.StatusCode())
	}
	resp, _ = app.Test("DELETE", "/users/7", nil)
	if resp.StatusCode() != 204 {
		t.Fatalf("empty -> 204, got %d", resp.StatusCode())
	}
	resp, _ = app.Test("GET", "/boom", nil)
	if resp.StatusCode() != 500 {
		t.Fatalf("boom: %d", resp.StatusCode())
	}
}

func TestHandle_BodyValidationAndStatus(t *testing.T) {
	app := thApp()
	resp, _ := app.Test("POST", "/users", strings.NewReader(`{"name":"Bia"}`))
	if resp.StatusCode() != 201 || !strings.Contains(string(resp.Body()), `"name":"Bia"`) {
		t.Fatalf("create: %d %s", resp.StatusCode(), resp.Body())
	}
	resp, _ = app.Test("POST", "/users", strings.NewReader(`{}`))
	if resp.StatusCode() != 422 && resp.StatusCode() != 400 {
		t.Fatalf("validation: %d %s", resp.StatusCode(), resp.Body())
	}
}

type thTagged struct {
	UserID int64  `param:"id"`
	Skip   string `param:"-"`
}

func TestHandle_ParamConventionAndOverrides(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})
	app.Get("/t/:id", Handle(func(ctx context.Context, in *thTagged) (*thUser, error) {
		return &thUser{ID: in.UserID, Name: in.Skip}, nil
	}))
	resp, _ := app.Test("GET", "/t/9", nil)
	if resp.StatusCode() != 200 || !strings.Contains(string(resp.Body()), `"id":9`) {
		t.Fatalf("explicit param tag override: %d %s", resp.StatusCode(), resp.Body())
	}
}
