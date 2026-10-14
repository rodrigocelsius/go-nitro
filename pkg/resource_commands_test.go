package nitro_test

import (
	"context"
	"encoding/json"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type bizOrder struct {
	nitro.Model
	CustomerID string  `json:"customer_id"`
	Status     string  `json:"status"`
	Total      float64 `json:"total"`
	Note       string  `json:"note,omitempty"`
}

type orderService struct {
	repo *nitro.MemoryRepository[bizOrder]
}

func (s *orderService) Place(ctx context.Context, in struct {
	CustomerID string  `json:"customer_id" validate:"required"`
	Total      float64 `json:"total" validate:"gt=0"`
}) (*bizOrder, error) {
	o := &bizOrder{CustomerID: in.CustomerID, Total: in.Total, Status: "draft"}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *orderService) Submit(ctx context.Context, id string) (*bizOrder, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.Status != "draft" {
		return nil, nitro.Conflict("not draft")
	}
	o.Status = "pending"
	return o, s.repo.Update(ctx, o)
}

func (s *orderService) Cancel(ctx context.Context, id string, in struct {
	Reason string `json:"reason" validate:"required"`
}) (*bizOrder, error) {
	o, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Status = "cancelled"
	o.Note = in.Reason
	return o, s.repo.Update(ctx, o)
}

func (s *orderService) Pending(ctx context.Context, page nitro.Page) ([]bizOrder, error) {
	_ = page
	all, err := s.repo.List(ctx, 1, 100)
	if err != nil {
		return nil, err
	}
	out := make([]bizOrder, 0)
	for _, item := range all.Items {
		var o *bizOrder
		switch v := item.(type) {
		case bizOrder:
			copy := v
			o = &copy
		case *bizOrder:
			o = v
		default:
			continue
		}
		if o != nil && o.Status == "pending" {
			out = append(out, *o)
		}
	}
	return out, nil
}

func TestResourceCommands_ItemAndCollection(t *testing.T) {
	repo := nitro.NewMemoryRepo[bizOrder]()
	svc := &orderService{repo: repo}
	app := nitro.New(nitro.Config{DisableDefaults: true})

	app.Resource("/orders", &bizOrder{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		NoCache: true,
		Rules: nitro.Rules(func(o *bizOrder) error {
			if o.Status == "cancelled" {
				return nitro.Conflict("cancelled")
			}
			return nil
		}),
		Commands: nitro.Commands(
			nitro.Cmd("place", svc.Place),
			nitro.Cmd("submit", svc.Submit),
			nitro.Cmd("cancel", svc.Cancel),
		),
		Queries: nitro.Queries(
			nitro.Q("pending", svc.Pending),
		),
	})

	tc := nitro.NewTestClient(app)

	place := tc.PostJSON(t, "/orders/place", map[string]any{
		"customer_id": "c1",
		"total":       100,
	})
	tc.AssertStatus(t, place, 201)
	id := jsonField(t, place.Body(), "id")

	submit := tc.Post(t, "/orders/"+id+"/submit", "")
	tc.AssertStatus(t, submit, 200)
	tc.AssertBodyContains(t, submit, `"status":"pending"`)

	cancel := tc.PostJSON(t, "/orders/"+id+"/cancel", map[string]string{
		"reason": "changed mind",
	})
	tc.AssertStatus(t, cancel, 200)

	conflict := tc.Post(t, "/orders/"+id+"/submit", "")
	tc.AssertStatus(t, conflict, 409)

	list := tc.Get(t, "/orders/queries/pending")
	tc.AssertStatus(t, list, 200)
}

func jsonField(t *testing.T, body []byte, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	v, _ := m[key].(string)
	if v == "" {
		t.Fatalf("missing %q in %s", key, body)
	}
	return v
}
