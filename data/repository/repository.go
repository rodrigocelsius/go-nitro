package repository

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

// Repository is the generic data-access interface.
type Repository[T any] interface {
	FindByID(ctx context.Context, id any) (*T, error)
	Find(ctx context.Context, q query.Query) ([]T, error)
	FindOne(ctx context.Context, q query.Query) (*T, error)
	Count(ctx context.Context, q query.Query) (int, error)
	List(ctx context.Context, q query.Query) (*query.PageResult[T], error)
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id any) error
}
