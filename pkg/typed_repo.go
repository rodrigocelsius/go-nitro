package nitro

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TypedRepository is the typed CRUD repository interface (Go generics).
// Prefer nitro.Resource(app, prefix, repo). Use AdaptRepo only with app.Resource / CacheRepo.
type TypedRepository[T any] interface {
	FindByID(ctx context.Context, id any) (*T, error)
	List(ctx context.Context, page, perPage int) (*ResourceListResult, error)
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id any) error
}

type typedListQueryable[T any] interface {
	ListQuery(context.Context, domain.ListQuery) (*domain.ResourceListResult, error)
}

type typedRepoAdapter[T any] struct {
	inner TypedRepository[T]
}

// typedQueryableRepoAdapter wraps repos that push List filters to SQL (BLinq).
// Only this type implements domain.QueryableLister — plain MemoryRepo must not,
// or Resource List would skip in-memory tenant/FilterRules post-filters.
type typedQueryableRepoAdapter[T any] struct {
	typedRepoAdapter[T]
}

// AdaptRepo adapts a TypedRepository[T] to domain.ResourceRepository.
// If the inner implements ListQuery, the result is also a QueryableLister.
func AdaptRepo[T any](repo TypedRepository[T]) domain.ResourceRepository {
	base := typedRepoAdapter[T]{inner: repo}
	if _, ok := repo.(typedListQueryable[T]); ok {
		return &typedQueryableRepoAdapter[T]{typedRepoAdapter: base}
	}
	return &base
}

func (a *typedRepoAdapter[T]) FindByID(ctx context.Context, id any) (any, error) {
	return a.inner.FindByID(ctx, id)
}

func (a *typedRepoAdapter[T]) List(ctx context.Context, page, perPage int) (*ResourceListResult, error) {
	return a.inner.List(ctx, page, perPage)
}

func (a *typedRepoAdapter[T]) Create(ctx context.Context, entity any) error {
	t, ok := entity.(*T)
	if !ok {
		return BadRequest("unsupported entity type")
	}
	return a.inner.Create(ctx, t)
}

func (a *typedRepoAdapter[T]) Update(ctx context.Context, entity any) error {
	t, ok := entity.(*T)
	if !ok {
		return BadRequest("unsupported entity type")
	}
	return a.inner.Update(ctx, t)
}

func (a *typedRepoAdapter[T]) Delete(ctx context.Context, id any) error {
	return a.inner.Delete(ctx, id)
}

func (a *typedQueryableRepoAdapter[T]) ListQuery(ctx context.Context, q domain.ListQuery) (*ResourceListResult, error) {
	return a.inner.(typedListQueryable[T]).ListQuery(ctx, q)
}
