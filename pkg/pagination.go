package nitro

// PaginationMeta holds computed pagination metadata.
type PaginationMeta struct {
	Total      int
	Page       int
	PerPage    int
	TotalPages int
	Offset     int
	End        int
}

// Paginate computes offset, end, and totalPages from page/perPage.
// Returns a ready-to-use struct.
//
// Example:
//
//	p := nitro.Paginate(len(users), 1, 20)
//	slice := nitro.PageSlice(users, p)
func Paginate(total, page, perPage int) PaginationMeta {
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	totalPages := (total + perPage - 1) / perPage
	offset := (page - 1) * perPage
	if offset > total {
		offset = total
	}
	end := offset + perPage
	if end > total {
		end = total
	}
	return PaginationMeta{
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Offset:     offset,
		End:        end,
	}
}

// PageSlice slices any slice according to pagination.
//
// Example:
//
//	all := repo.GetAll()
//	p := nitro.Paginate(len(all), page, perPage)
//	pageItems := nitro.PageSlice(all, p)
func PageSlice[T any](items []T, p PaginationMeta) []T {
	if p.Offset >= len(items) {
		return []T{}
	}
	end := p.End
	if end > len(items) {
		end = len(items)
	}
	return items[p.Offset:end]
}

// ToAny converts a typed slice to []any.
// Useful to populate ResourceListResult.Items.
//
// Example:
//
//	return &nitro.ResourceListResult{
//	    Items: nitro.ToAny(nitro.PageSlice(todos, p)),
//	    Total: p.Total,
//	}
func ToAny[T any](items []T) []any {
	out := make([]any, len(items))
	for i, v := range items {
		out[i] = v
	}
	return out
}

// ListResult paginates a slice and returns a *ResourceListResult ready for the API.
//
// Example:
//
//	return nitro.ListResult(allTodos, page, perPage), nil
func ListResult[T any](items []T, page, perPage int) *ResourceListResult {
	p := Paginate(len(items), page, perPage)
	return &ResourceListResult{
		Items:      ToAny(PageSlice(items, p)),
		Total:      p.Total,
		Page:       p.Page,
		PerPage:    p.PerPage,
		TotalPages: p.TotalPages,
	}
}
