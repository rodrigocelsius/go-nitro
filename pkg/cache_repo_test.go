package nitro

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type cacheProbeRepo struct {
	gets int
}

func (r *cacheProbeRepo) FindByID(ctx context.Context, id any) (any, error) {
	r.gets++
	return map[string]any{"id": id}, nil
}
func (r *cacheProbeRepo) List(ctx context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	return &domain.ResourceListResult{Items: nil, Total: 0, Page: page, PerPage: perPage}, nil
}
func (r *cacheProbeRepo) Create(ctx context.Context, entity any) error { return nil }
func (r *cacheProbeRepo) Update(ctx context.Context, entity any) error { return nil }
func (r *cacheProbeRepo) Delete(ctx context.Context, id any) error     { return nil }

func TestApplyResourceCache_Default(t *testing.T) {
	inner := &cacheProbeRepo{}
	wrapped := applyResourceCache(inner, (&ResourceConfig{}).toOpts())
	if !IsCacheRepo(wrapped) {
		t.Fatal("expected default cache wrap")
	}
	_, _ = wrapped.FindByID(context.Background(), "1")
	_, _ = wrapped.FindByID(context.Background(), "1")
	if inner.gets != 1 {
		t.Fatalf("cache miss count = %d, want 1", inner.gets)
	}
}

func TestApplyResourceCache_WithoutCache(t *testing.T) {
	inner := &cacheProbeRepo{}
	wrapped := applyResourceCache(inner, (&ResourceConfig{NoCache: true}).toOpts())
	if IsCacheRepo(wrapped) {
		t.Fatal("WithoutCache should not wrap")
	}
}

func TestApplyResourceCache_AlreadyCached(t *testing.T) {
	inner := &cacheProbeRepo{}
	first := CacheRepo(inner, time.Minute)
	second := applyResourceCache(first, (&ResourceConfig{CacheTTL: 10 * time.Minute}).toOpts())
	if first != second {
		t.Fatal("should not double-wrap CacheRepo")
	}
}

func TestResourceConfig_ZeroCacheTTLUsesDefault(t *testing.T) {
	o := (&ResourceConfig{}).toOpts()
	if o.CacheTTL != DefaultCacheTTL {
		t.Fatalf("CacheTTL = %v, want %v", o.CacheTTL, DefaultCacheTTL)
	}
}

type cacheCountRepo struct {
	lists   int
	updates int
	creates int
	deletes int
	gets    int
}

func (r *cacheCountRepo) FindByID(ctx context.Context, id any) (any, error) {
	r.gets++
	return map[string]any{"id": id}, nil
}
func (r *cacheCountRepo) List(ctx context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	r.lists++
	return &domain.ResourceListResult{
		Items:   []any{map[string]any{"n": r.lists}},
		Total:   1,
		Page:    page,
		PerPage: perPage,
	}, nil
}
func (r *cacheCountRepo) Create(ctx context.Context, entity any) error {
	r.creates++
	return nil
}
func (r *cacheCountRepo) Update(ctx context.Context, entity any) error {
	r.updates++
	return nil
}
func (r *cacheCountRepo) Delete(ctx context.Context, id any) error {
	r.deletes++
	return nil
}

func TestCacheRepo_ListCachesAndPurgeOnUpdate(t *testing.T) {
	inner := &cacheCountRepo{}
	repo := CacheRepo(inner, time.Minute)

	a, err := repo.List(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	b, err := repo.List(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if inner.lists != 1 {
		t.Fatalf("List hits = %d, want 1", inner.lists)
	}
	if a != b {
		// same pointer from cache is fine; at least totals match
		if a.Total != b.Total {
			t.Fatalf("cached list mismatch")
		}
	}

	if err := repo.Update(context.Background(), map[string]any{"id": "1"}); err != nil {
		t.Fatal(err)
	}
	if inner.updates != 1 {
		t.Fatalf("updates = %d", inner.updates)
	}
	_, _ = repo.List(context.Background(), 1, 10)
	if inner.lists != 2 {
		t.Fatalf("after Update purge List hits = %d, want 2", inner.lists)
	}
}

func TestCacheRepo_PurgeOnCreateAndDelete(t *testing.T) {
	inner := &cacheCountRepo{}
	repo := CacheRepo(inner, time.Minute)
	_, _ = repo.FindByID(context.Background(), "1")
	_, _ = repo.FindByID(context.Background(), "1")
	if inner.gets != 1 {
		t.Fatalf("gets = %d", inner.gets)
	}
	_ = repo.Create(context.Background(), map[string]any{"id": "2"})
	_, _ = repo.FindByID(context.Background(), "1")
	if inner.gets != 2 {
		t.Fatalf("after Create purge gets = %d, want 2", inner.gets)
	}
	_ = repo.Delete(context.Background(), "1")
	_, _ = repo.FindByID(context.Background(), "1")
	if inner.gets != 3 {
		t.Fatalf("after Delete purge gets = %d, want 3", inner.gets)
	}
}
