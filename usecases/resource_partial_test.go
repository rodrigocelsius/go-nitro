package usecases

import (
	"context"
	"reflect"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type partialBase struct {
	ID        string
	UpdatedAt int
}

type partialEntity struct {
	partialBase
	Name   string
	Count  int
	Tags   []string
	hidden int
}

type partialRepo struct {
	fullUpdates int
	fields      []string
}

func (r *partialRepo) FindByID(context.Context, any) (any, error) { return nil, nil }
func (r *partialRepo) List(context.Context, int, int) (*domain.ResourceListResult, error) {
	return nil, nil
}
func (r *partialRepo) Create(context.Context, any) error { return nil }
func (r *partialRepo) Update(context.Context, any) error { r.fullUpdates++; return nil }
func (r *partialRepo) Delete(context.Context, any) error { return nil }
func (r *partialRepo) UpdateFields(_ context.Context, _ any, fields []string) error {
	r.fields = fields
	return nil
}

func TestChangedFieldNames(t *testing.T) {
	before := &partialEntity{partialBase: partialBase{ID: "1", UpdatedAt: 1}, Name: "a", Count: 1, Tags: []string{"x"}}
	after := &partialEntity{partialBase: partialBase{ID: "1", UpdatedAt: 2}, Name: "b", Count: 1, Tags: []string{"x", "y"}, hidden: 9}
	got := changedFieldNames(before, after)
	if !reflect.DeepEqual(got, []string{"Name", "Tags"}) {
		t.Fatalf("changed = %v", got)
	}
}

func TestPersistUpdate_UsesPartialWhenMerging(t *testing.T) {
	repo := &partialRepo{}
	before := &partialEntity{Name: "a"}
	after := &partialEntity{Name: "b"}
	if err := persistUpdate(context.Background(), repo, before, after, true); err != nil {
		t.Fatal(err)
	}
	if repo.fullUpdates != 0 || !reflect.DeepEqual(repo.fields, []string{"Name"}) {
		t.Fatalf("full=%d fields=%v", repo.fullUpdates, repo.fields)
	}
}

func TestPersistUpdate_NoChangeNoWrite(t *testing.T) {
	repo := &partialRepo{}
	e := &partialEntity{Name: "a"}
	if err := persistUpdate(context.Background(), repo, e, &partialEntity{Name: "a"}, true); err != nil {
		t.Fatal(err)
	}
	if repo.fullUpdates != 0 || repo.fields != nil {
		t.Fatalf("expected no write: full=%d fields=%v", repo.fullUpdates, repo.fields)
	}
}

func TestPersistUpdate_ReplaceModeIsFull(t *testing.T) {
	repo := &partialRepo{}
	if err := persistUpdate(context.Background(), repo, &partialEntity{}, &partialEntity{Name: "b"}, false); err != nil {
		t.Fatal(err)
	}
	if repo.fullUpdates != 1 {
		t.Fatalf("replace mode must use full Update")
	}
}
