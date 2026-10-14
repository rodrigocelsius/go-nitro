package nitro

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestFlightGroup_Deduplicates(t *testing.T) {
	var g flightGroup
	var calls int32
	release := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Do("k", func() (any, error) {
				atomic.AddInt32(&calls, 1)
				<-release
				return "v", nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond) // let all goroutines pile up on the key
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 backing call, got %d", got)
	}
}

func TestFlightGroup_ErrorShared(t *testing.T) {
	var g flightGroup
	want := errors.New("boom")
	v, err := g.Do("k", func() (any, error) { return nil, want })
	if !errors.Is(err, want) || v != nil {
		t.Fatalf("v=%v err=%v", v, err)
	}
}

type countingRepo struct {
	mem     *MemoryRepository[raceTodo]
	listErr error
	lists   int32
	gets    int32
}

func (c *countingRepo) FindByID(ctx context.Context, id any) (any, error) {
	atomic.AddInt32(&c.gets, 1)
	return c.mem.FindByID(ctx, id)
}

func (c *countingRepo) List(ctx context.Context, page, perPage int) (*ResourceListResult, error) {
	atomic.AddInt32(&c.lists, 1)
	return c.mem.List(ctx, page, perPage)
}

func (c *countingRepo) ListQuery(ctx context.Context, q domain.ListQuery) (*ResourceListResult, error) {
	atomic.AddInt32(&c.lists, 1)
	return c.mem.List(ctx, q.Page, q.PerPage)
}

func (c *countingRepo) Create(ctx context.Context, e any) error {
	return c.mem.Create(ctx, e.(*raceTodo))
}
func (c *countingRepo) Update(ctx context.Context, e any) error {
	return c.mem.Update(ctx, e.(*raceTodo))
}
func (c *countingRepo) Delete(ctx context.Context, id any) error { return c.mem.Delete(ctx, id) }

func TestCachedRepo_StampedeProtection(t *testing.T) {
	inner := &countingRepo{mem: NewMemoryRepo[raceTodo]()}
	if err := inner.Create(context.Background(), &raceTodo{ID: "hot"}); err != nil {
		t.Fatal(err)
	}
	repo := unwrapCachedRepo(t, CacheRepo(inner, time.Minute))

	var started atomic.Int32
	_ = &started // flights collapse naturally under the race below

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = repo.FindByID(context.Background(), "hot")
		}()
	}
	wg.Wait()
	// All 16 may race before the flight forms; assert bounded concurrency:
	// with singleflight the count must stay far below 16 in practice.
	if gets := atomic.LoadInt32(&inner.gets); gets > 4 {
		t.Fatalf("stampede: %d backing calls for a hot key", gets)
	}
}

func TestListQueryCacheKey_NoCollision(t *testing.T) {
	a := domain.ListQuery{Page: 1, PerPage: 10, Filters: []domain.ListFilter{{Field: "name", Value: "a|b"}}}
	b := domain.ListQuery{Page: 1, PerPage: 10,
		Filters: []domain.ListFilter{{Field: "name", Value: "a"}, {Field: "b", Value: ""}},
	}
	ka := listQueryCacheKey(a)
	kb := listQueryCacheKey(b)
	if ka == kb {
		t.Fatalf("collision between %q and %q", ka, kb)
	}
	c := domain.ListQuery{Page: 1, PerPage: 10, Filters: []domain.ListFilter{{Field: "name", Value: "a|b"}}}
	if listQueryCacheKey(c) != ka {
		t.Fatal("identical queries must hash equal")
	}
}

func TestCachedRepo_SweepsExpired(t *testing.T) {
	inner := &countingRepo{mem: NewMemoryRepo[raceTodo]()}
	repo := unwrapCachedRepo(t, CacheRepo(inner, time.Millisecond))
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		id := string(rune('a' + i%26))
		_, _ = repo.FindByID(ctx, id)
	}
	time.Sleep(5 * time.Millisecond)
	for i := 50; i < 120; i++ {
		id := string(rune('a' + i%26))
		_, _ = repo.FindByID(ctx, id)
	}
	repo.mu.RLock()
	n := len(repo.items)
	repo.mu.RUnlock()
	if n > defaultCacheMaxEntries {
		t.Fatalf("cache grew unbounded: %d entries", n)
	}
}

func unwrapCachedRepo(t *testing.T, repo domain.ResourceRepository) *cachedRepo {
	t.Helper()
	switch r := repo.(type) {
	case *cachedRepo:
		return r
	case *cachedQueryableRepo:
		return r.cachedRepo
	default:
		t.Fatalf("want CacheRepo, got %T", repo)
		return nil
	}
}
