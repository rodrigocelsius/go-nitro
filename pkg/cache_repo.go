package nitro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// DefaultCacheTTL is the cache duration of the shared (App cache)
// read-through and of CacheRepo / cache Set calls given no TTL. The
// per-process Resource cache has no default: set ResourceConfig.CacheTTL.
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

// applyResourceCache wraps repo with CacheRepo when opts.CacheTTL is
// positive. The per-process cache is opt-in: with several replicas (or
// writes outside the Resource) it serves stale rows nobody asked for.
// Skips wrap if repo is already a CacheRepo.
func applyResourceCache(repo domain.ResourceRepository, opts resourceOpts) domain.ResourceRepository {
	if repo == nil || IsCacheRepo(repo) {
		return repo
	}
	if opts.CacheTTL <= 0 {
		return repo
	}
	return CacheRepo(repo, opts.CacheTTL)
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

// listQueryCacheKey fingerprints page/filters/sort. Every component —
// including the filter operator: ?price_gte=10 and ?price_lt=10 are
// different queries — is length-prefixed, so distinct queries can never
// produce the same input, and the digest is SHA-256: the key also carries
// the tenant filter next to client-controlled values, so a forgeable hash
// (FNV) would let one tenant aim for another tenant's cached page.
func listQueryCacheKey(q domain.ListQuery) string {
	var b strings.Builder
	part := func(s string) {
		b.WriteString(strconv.Itoa(len(s)))
		b.WriteByte(':')
		b.WriteString(s)
	}
	b.WriteString(strconv.Itoa(q.Page))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(q.PerPage))
	for _, f := range q.Filters {
		b.WriteByte('f')
		part(f.Field)
		part(strings.ToLower(f.Op))
		part(f.Value)
	}
	for _, s := range q.Sort {
		if s.Desc {
			b.WriteByte('d')
		} else {
			b.WriteByte('a')
		}
		part(s.Field)
	}
	// Keyset pages are different results from offset pages, and each
	// cursor position is its own page.
	if q.Keyset {
		b.WriteByte('k')
		if q.After != nil {
			part(q.After.ID)
			for _, v := range q.After.Values {
				part(v)
			}
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "lq:" + hex.EncodeToString(sum[:])
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
	// Evict down to 7/8 of the cap, not just to the cap: evicting a single
	// entry meant a full sort on every store once the cache was full (one
	// per unique ?filter combination — attacker-controlled).
	excess := len(live) - r.maxEntries*7/8
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
