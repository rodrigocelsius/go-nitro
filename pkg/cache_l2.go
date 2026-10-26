package nitro

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// l2CachedRepo is the Resource read-through cache over a shared L2 cache
// (App.SetCache(nitro.RedisCache(…))): every replica reads and fills the
// same entries, and writes bust them through CacheBustPattern (OnMutate).
//
// Entities are stored as JSON. Fields hidden from JSON (`json:"-"`, e.g. a
// password hash) are therefore absent from cached copies: write paths read
// with domain.BypassCache, so a PATCH merge never writes them back empty.
//
// The L2 is an optimization, never a dependency: when it errors (Redis down,
// timeout, undecodable entry) reads go straight to the repository and the
// failure is logged at most once per minute; the cache is then skipped for
// l2RetryAfter so a down Redis does not add its timeout to every request.
type l2CachedRepo struct {
	inner   domain.ResourceRepository
	l2      Cache
	prefix  string // "drivers:" — matches the "drivers:*" bust pattern
	ttl     time.Duration
	flights flightGroup
	lastLog atomic.Int64
	model   atomic.Pointer[reflect.Type] // entity struct type, for decoding
	// downUntil (unix nanos) opens a circuit after an L2 failure: requests skip
	// the cache instead of each waiting for a dial/read timeout.
	downUntil atomic.Int64
}

// l2RetryAfter is how long the L2 is skipped after a failure.
const l2RetryAfter = 30 * time.Second

type l2CachedQueryableRepo struct {
	*l2CachedRepo
}

// l2Envelope is the cached form of an entity or a list page.
type l2Envelope struct {
	Items      []json.RawMessage
	Ptr        bool // items were *T (Store) rather than T
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// L2CacheRepo wraps repo with a read-through cache on c (typically Redis).
// keyPrefix scopes the entries ("drivers:"); writes must bust keyPrefix+"*".
// Resource wires it automatically when the App has a cache (SetCache).
func L2CacheRepo(repo domain.ResourceRepository, c Cache, keyPrefix string, ttl time.Duration) domain.ResourceRepository {
	if repo == nil || c == nil {
		return repo
	}
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	base := &l2CachedRepo{inner: repo, l2: c, prefix: keyPrefix, ttl: ttl}
	return wrapL2(base)
}

// newResourceL2Cache is L2CacheRepo with the model type known up front.
func newResourceL2Cache(repo domain.ResourceRepository, c Cache, keyPrefix string, ttl time.Duration, model reflect.Type) domain.ResourceRepository {
	if repo == nil || c == nil {
		return repo
	}
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	base := &l2CachedRepo{inner: repo, l2: c, prefix: keyPrefix, ttl: ttl}
	base.rememberModel(reflect.New(model).Interface())
	return wrapL2(base)
}

func wrapL2(base *l2CachedRepo) domain.ResourceRepository {
	repo := base.inner
	if _, ok := repo.(domain.QueryableLister); ok {
		return &l2CachedQueryableRepo{l2CachedRepo: base}
	}
	return base
}

// InTx forwards domain.Transactor to the inner repository.
func (r *l2CachedRepo) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return domain.RunInTx(ctx, r.inner, fn)
}

func (r *l2CachedRepo) FindByID(ctx context.Context, id any) (any, error) {
	if domain.CacheBypassed(ctx) {
		return r.inner.FindByID(ctx, id)
	}
	key := r.prefix + "id:" + IDKey(id)
	if v, ok := r.get(key); ok && len(v.Items) == 1 {
		if e, err := decodeL2Item(v.Items[0], v.Ptr, r.modelOf()); err == nil {
			return e, nil
		}
	}
	return r.flights.Do(key, func() (any, error) {
		e, err := r.inner.FindByID(ctx, id)
		if err != nil || e == nil {
			return e, err
		}
		r.set(key, []any{e}, nil)
		return e, nil
	})
}

func (r *l2CachedRepo) List(ctx context.Context, page, perPage int) (*ResourceListResult, error) {
	key := r.prefix + "list:" + strconv.Itoa(page) + ":" + strconv.Itoa(perPage)
	return r.list(ctx, key, func() (*ResourceListResult, error) { return r.inner.List(ctx, page, perPage) })
}

func (r *l2CachedQueryableRepo) ListQuery(ctx context.Context, q domain.ListQuery) (*ResourceListResult, error) {
	ql := r.inner.(domain.QueryableLister)
	key := r.prefix + listQueryCacheKey(q)
	return r.list(ctx, key, func() (*ResourceListResult, error) { return ql.ListQuery(ctx, q) })
}

func (r *l2CachedRepo) list(ctx context.Context, key string, load func() (*ResourceListResult, error)) (*ResourceListResult, error) {
	if domain.CacheBypassed(ctx) {
		return load()
	}
	if v, ok := r.get(key); ok {
		if res, err := decodeL2List(v, r.modelOf()); err == nil {
			return res, nil
		}
	}
	out, err := r.flights.Do(key, func() (any, error) {
		res, err := load()
		if err != nil || res == nil {
			return res, err
		}
		r.set(key, res.Items, res)
		return res, nil
	})
	if err != nil || out == nil {
		return nil, err
	}
	return out.(*ResourceListResult), nil
}

func (r *l2CachedRepo) Create(ctx context.Context, entity any) error {
	r.rememberModel(entity)
	return r.inner.Create(ctx, entity)
}

func (r *l2CachedRepo) Update(ctx context.Context, entity any) error {
	return r.inner.Update(ctx, entity)
}

// UpdateFields forwards partial updates (falls back to a full Update).
func (r *l2CachedRepo) UpdateFields(ctx context.Context, entity any, fields []string) error {
	if pu, ok := r.inner.(domain.ResourcePartialUpdater); ok {
		return pu.UpdateFields(ctx, entity, fields)
	}
	return r.inner.Update(ctx, entity)
}

func (r *l2CachedRepo) Delete(ctx context.Context, id any) error {
	return r.inner.Delete(ctx, id)
}

// rememberModel records the entity type (decoding needs it and
// ResourceRepository is untyped); L2CacheRepo learns it from the first
// entity it sees, Resource passes it up front.
func (r *l2CachedRepo) rememberModel(e any) {
	if r.model.Load() != nil {
		return
	}
	t := reflect.TypeOf(e)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t != nil && t.Kind() == reflect.Struct {
		r.model.CompareAndSwap(nil, &t)
	}
}

func (r *l2CachedRepo) modelOf() reflect.Type {
	if t := r.model.Load(); t != nil {
		return *t
	}
	return nil
}

func (r *l2CachedRepo) available() bool {
	return time.Now().UnixNano() >= r.downUntil.Load()
}

func (r *l2CachedRepo) get(key string) (*l2Envelope, bool) {
	if !r.available() {
		return nil, false
	}
	var env l2Envelope
	if err := r.l2.Get(key, &env); err != nil {
		var miss domain.ErrCacheMiss
		var missPtr *domain.ErrCacheMiss
		if !errors.As(err, &miss) && !errors.As(err, &missPtr) {
			r.warn("get", err)
		}
		return nil, false
	}
	return &env, true
}

func (r *l2CachedRepo) set(key string, items []any, res *ResourceListResult) {
	if !r.available() {
		return
	}
	env := l2Envelope{Items: make([]json.RawMessage, 0, len(items))}
	for _, it := range items {
		r.rememberModel(it)
		v := reflect.ValueOf(it)
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			env.Ptr = true
			v = v.Elem()
		}
		raw, err := json.Marshal(v.Interface())
		if err != nil {
			r.warn("encode", err) // not cacheable: serve uncached
			return
		}
		env.Items = append(env.Items, raw)
	}
	if res != nil {
		env.Total, env.Page, env.PerPage, env.TotalPages = res.Total, res.Page, res.PerPage, res.TotalPages
	}
	if err := r.l2.Set(key, env, r.ttl); err != nil {
		r.warn("set", err)
	}
}

func (r *l2CachedRepo) warn(op string, err error) {
	if op == "get" || op == "set" {
		r.downUntil.Store(time.Now().Add(l2RetryAfter).UnixNano())
	}
	now := time.Now().Unix()
	last := r.lastLog.Load()
	if now-last < 60 || !r.lastLog.CompareAndSwap(last, now) {
		return
	}
	log.Printf("nitro: resource cache %s %q failed, serving from the repository: %v", op, r.prefix, err)
}

func decodeL2Item(raw json.RawMessage, ptr bool, model reflect.Type) (any, error) {
	if model == nil {
		return nil, fmt.Errorf("nitro: l2 cache: model type unknown")
	}
	pv := reflect.New(model)
	if err := json.Unmarshal(raw, pv.Interface()); err != nil {
		return nil, err
	}
	if ptr {
		return pv.Interface(), nil
	}
	return pv.Elem().Interface(), nil
}

func decodeL2List(env *l2Envelope, model reflect.Type) (*ResourceListResult, error) {
	items := make([]any, len(env.Items))
	for i, raw := range env.Items {
		it, err := decodeL2Item(raw, env.Ptr, model)
		if err != nil {
			return nil, err
		}
		items[i] = it
	}
	return &ResourceListResult{
		Items:      items,
		Total:      env.Total,
		Page:       env.Page,
		PerPage:    env.PerPage,
		TotalPages: env.TotalPages,
	}, nil
}
