package domain

import "context"

// ResourceRepository defines the CRUD operations required by app.Resource().
// Any repository that implements this interface can be used with Resource().
type ResourceRepository interface {
	// FindByID returns a single entity by its ID.
	FindByID(ctx context.Context, id any) (any, error)

	// List returns a paginated list of entities.
	List(ctx context.Context, page, perPage int) (*ResourceListResult, error)

	// Create inserts a new entity.
	Create(ctx context.Context, entity any) error

	// Update updates an existing entity.
	Update(ctx context.Context, entity any) error

	// Delete removes an entity by its ID.
	Delete(ctx context.Context, id any) error
}

// ResourcePartialUpdater is optionally implemented by repositories that can
// persist a subset of fields. Resource PATCH uses it to write only the fields
// the request changed (plus UpdatedAt), so columns written concurrently by
// other code paths or services are not overwritten with stale values
// (lost update). fields are Go struct field names, promoted fields included.
type ResourcePartialUpdater interface {
	UpdateFields(ctx context.Context, entity any, fields []string) error
}

// ResourceListResult is the paginated result returned by List.
type ResourceListResult struct {
	Items      []any `json:"items"`
	Total      int   `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
	// NextCursor is set on Resources with cursor paging when there is a
	// next page: pass it as ?cursor= to continue without OFFSET or COUNT.
	NextCursor string `json:"next_cursor,omitempty"`
}

// ListFilter is a filter from query string (?field=value).
// Op: eq|gte|lte|gt|lt|contains|icontains; empty defaults to eq.
type ListFilter struct {
	Field string
	Value string
	Op    string
}

// ListSort is ORDER BY from ?sort=field or ?sort=-field.
type ListSort struct {
	Field string
	Desc  bool
}

// ListCursor is a keyset position: the row a page ended at.
type ListCursor struct {
	// Values are that row's values of the ListQuery.Sort fields, in order.
	Values []string
	// ID is that row's primary key (the final tiebreaker).
	ID string
}

// ListQuery carries pagination + filters/sort for SQL push-down (BLinq).
type ListQuery struct {
	Page    int
	PerPage int
	Filters []ListFilter
	Sort    []ListSort

	// Keyset asks for cursor (keyset) paging instead of OFFSET: return up
	// to PerPage rows ordered by Sort then primary key, starting after
	// After (nil = from the first row), without counting the total. A
	// repository that honors it answers with Page == 0; one that does not
	// know keyset paging keeps answering Page >= 1 and the Resource
	// rejects the cursor request instead of looping on the first page.
	Keyset bool
	After  *ListCursor
}

// QueryableLister is optional: Resource List pushes filters to the repo (BLinq/SQL)
// instead of fetching a large window and filtering in-memory.
type QueryableLister interface {
	ListQuery(ctx context.Context, q ListQuery) (*ResourceListResult, error)
}

// AsResourceRepository allows typed stores (e.g. *pgx.Store[T]) to be passed to
// app.Resource without an explicit AdaptStore call.
type AsResourceRepository interface {
	AsResourceRepository() ResourceRepository
}

// ResourceDB creates a ResourceRepository from a model (e.g. *pgx.DB).
// Allows: api.Resource("/drivers", db, &Driver{}, &ResourceConfig{...}).
type ResourceDB interface {
	ResourceRepository(model any, table string) ResourceRepository
}
