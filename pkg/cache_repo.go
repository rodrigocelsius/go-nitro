package nitro

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// DefaultCacheTTL is the cache duration applied by Resource when CacheTTL is unset.
const DefaultCacheTTL = 5 * time.Minute

// defaultCacheMaxEntries bounds the per-repo entry count. When exceeded,
// expired entries are swept first; if still over, soonest-to-expire are evicted.
const defaultCacheMaxEntries = 1024

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

type cachedRepo struct {
	inner      domain.ResourceRepository
	ttl        time.Duration
	flights    flightGroup
	mu         sync.RWMutex
	items      map[string]cacheEntry
	maxEntries int
}

// CacheRepo wraps a ResourceRepository with in-memory cache and TTL.
// Mutations (Create/Update/Delete) invalidate the cache; concurrent misses for
// the same key collapse into a single backing call (singleflight).
//
// Values handed out are shared with the cache — treat them as read-only.
//
// Prefer ResourceConfig.CacheTTL for generated CRUD; use CacheRepo
// when you want to wrap a repo explicitly (e.g. app.Resource + AdaptRepo).
//
//	app.Resource("/todos", &Todo{}, nitro.CacheRepo(repo, time.Minute))
//	app.Resource("/todos", &Todo{}, &nitro.ResourceConfig{CacheTTL: 10 * time.Minute})
//
// QueryableLister is only advertised when the inner repo implements it, so
// MemoryRepo + TenantScope still post-filters in Resource List.
func CacheRepo(repo domain.ResourceRepository, ttl time.Duration) domain.ResourceRepository {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	base := &cachedRepo{
		inner:      repo,
		ttl:        ttl,
		items:      make(map[string]cacheEntry),
		maxEntries: defaultCacheMaxEntries,
	}
	if _, ok := repo.(domain.QueryableLister); ok {
		return &cachedQueryableRepo{cachedRepo: base}
	}
	return base
}

// cachedQueryableRepo is CacheRepo when the inner pushes List filters (BLinq).
type cachedQueryableRepo struct {
	*cachedRepo
}

// IsCacheRepo reports whether repo is already wrapped by CacheRepo.
func IsCacheRepo(repo domain.ResourceRepository) bool {
	switch repo.(type) {
	case *cachedRepo, *cachedQueryableRepo:
		return true
	default:
		return false
	}
}

// applyResourceCache wraps repo with CacheRepo using opts.CacheTTL.
// Negative CacheTTL disables cache. Zero uses DefaultCacheTTL.
// Skips wrap if repo is already a CacheRepo.
func applyResourceCache(repo domain.ResourceRepository, opts resourceOpts) domain.ResourceRepository {
	if repo == nil || IsCacheRepo(repo) {
		return repo
	}
	ttl := opts.CacheTTL
	if ttl < 0 {
		return repo
	}
	if ttl == 0 {
		ttl = DefaultCacheTTL
	}
	return CacheRepo(repo, ttl)
}

// InTx forwards domain.Transactor to the inner repository.
func (r *cachedRepo) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return domain.RunInTx(ctx, r.inner, fn)
}

func (r *cachedRepo) FindByID(ctx context.Context, id any) (any, error) {
	if domain.CacheBypassed(ctx) {
		// Inside a unit of work / write path read the row the transaction
		// sees, never a cached copy (and never cache uncommitted state).
		return r.inner.FindByID(ctx, id)
	}
	key := "id:" + IDKey(id)
	if v, ok := r.lookup(key); ok {
		return v, nil
	}
	return r.flights.Do("fid:"+key, func() (any, error) {
		if v, ok := r.lookup(key); ok {
			return v, nil // populated while waiting for the flight
		}
		v, err := r.inner.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		r.store(key, v)
		return v, nil
	})
}

func (r *cachedRepo) List(ctx context.Context, page, perPage int) (*ResourceListResult, error) {
	key := fmt.Sprintf("list:%d:%d", page, perPage)
	if v, ok := r.lookupList(key); ok {
		return v, nil
	}
	v, err := r.flights.Do("list:"+key, func() (any, error) {
		if v, ok := r.lookupList(key); ok {
			return v, nil
		}
		res, err := r.inner.List(ctx, page, perPage)
		if err != nil {
			return nil, err
		}
		r.store(key, res)
		return res, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*ResourceListResult), nil
}

// ListQuery forwards filters to the inner (BLinq push-down) and caches the result.
func (r *cachedQueryableRepo) ListQuery(ctx context.Context, q domain.ListQuery) (*ResourceListResult, error) {
	ql := r.inner.(domain.QueryableLister)
	key := listQueryCacheKey(q)
	if v, ok := r.lookupList(key); ok {
		return v, nil
	}
	v, err := r.flights.Do("lq:"+key, func() (any, error) {
		if v, ok := r.lookupList(key); ok {
			return v, nil
		}
		res, err := ql.ListQuery(ctx, q)
		if err != nil {
			return nil, err
		}
		r.store(key, res)
		return res, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*ResourceListResult), nil
}

// listQueryCacheKey fingerprints page/filters/sort with NUL separators and a
// final hash, so distinct filter sets can never collide and key memory stays
// bounded for long filter values.
func listQueryCacheKey(q domain.ListQuery) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d:%d", q.Page, q.PerPage)
	for _, f := range q.Filters {
		fmt.Fprintf(&b, "\x00f\x00%s\x00%s", f.Field, f.Value)
	}
	for _, s := range q.Sort {
		dir := byte('a')
		if s.Desc {
			dir = 'd'
		}
		fmt.Fprintf(&b, "\x00s\x00%s\x00%c", s.Field, dir)
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(b.String()))
	return fmt.Sprintf("lq:%08x", h.Sum64())
}

func (r *cachedRepo) Create(ctx context.Context, entity any) error {
	err := r.inner.Create(ctx, entity)
	if err == nil {
		r.purgeAfterWrite(ctx)
	}
	return err
}

func (r *cachedRepo) Update(ctx context.Context, entity any) error {
	err := r.inner.Update(ctx, entity)
	if err == nil {
		r.purgeAfterWrite(ctx)
	}
	return err
}

// UpdateFields forwards partial updates when the inner repository supports
// them (falls back to a full Update otherwise).
func (r *cachedRepo) UpdateFields(ctx context.Context, entity any, fields []string) error {
	var err error
	if pu, ok := r.inner.(domain.ResourcePartialUpdater); ok {
		err = pu.UpdateFields(ctx, entity, fields)
	} else {
		err = r.inner.Update(ctx, entity)
	}
	if err == nil {
		r.purgeAfterWrite(ctx)
	}
	return err
}

func (r *cachedRepo) Delete(ctx context.Context, id any) error {
	err := r.inner.Delete(ctx, id)
	if err == nil {
		r.purgeAfterWrite(ctx)
	}
	return err
}

// lookup returns a live entry, lazily deleting expired ones.
func (r *cachedRepo) lookup(key string) (any, bool) {
	r.mu.RLock()
	e, ok := r.items[key]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		r.mu.Lock()
		if cur, still := r.items[key]; still && !time.Now().Before(cur.expiresAt) {
			delete(r.items, key)
		}
		r.mu.Unlock()
		return nil, false
	}
	return e.value, true
}

func (r *cachedRepo) lookupList(key string) (*ResourceListResult, bool) {
	v, ok := r.lookup(key)
	if !ok {
		return nil, false
	}
	res, ok := v.(*ResourceListResult)
	return res, ok
}

func (r *cachedRepo) store(key string, value any) {
	r.mu.Lock()
	r.items[key] = cacheEntry{value: value, expiresAt: time.Now().Add(r.ttl)}
	if len(r.items) > r.maxEntries {
		r.sweepLocked()
	}
	r.mu.Unlock()
}

// sweepLocked evicts expired entries and, if the map is still over budget,
// soonest-to-expire live entries. Caller holds r.mu (write).
func (r *cachedRepo) sweepLocked() {
	now := time.Now()
	for k, e := range r.items {
		if now.After(e.expiresAt) {
			delete(r.items, k)
		}
	}
	if len(r.items) <= r.maxEntries {
		return
	}
	type kv struct {
		key string
		exp time.Time
	}
	live := make([]kv, 0, len(r.items))
	for k, e := range r.items {
		live = append(live, kv{k, e.expiresAt})
	}
	sort.Slice(live, func(i, j int) bool { return live[i].exp.Before(live[j].exp) })
	excess := len(live) - r.maxEntries
	for i := 0; i < excess; i++ {
		delete(r.items, live[i].key)
	}
}

// purgeAfterWrite purges now and again after COMMIT when ctx is in a tx, so
// a read racing the commit cannot leave pre-transaction rows cached.
func (r *cachedRepo) purgeAfterWrite(ctx context.Context) {
	r.purge()
	if domain.InTx(ctx) {
		domain.AfterCommit(ctx, func(context.Context) { r.purge() })
	}
}

func (r *cachedRepo) purge() {
	r.mu.Lock()
	r.items = make(map[string]cacheEntry)
	r.mu.Unlock()
}
