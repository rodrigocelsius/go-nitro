package query

import (
	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
)

// InsertInto starts INSERT.
func InsertInto(e field.Entity) Query {
	q := Query{ast: expr.NewInsert(e.Ref())}
	q.softCol = e.SoftColumn
	q.tenantCol = e.TenantColumn
	return q
}

// UpsertInto starts UPSERT (INSERT … ON CONFLICT).
func UpsertInto(e field.Entity) Query {
	q := Query{ast: expr.NewUpsert(e.Ref())}
	q.softCol = e.SoftColumn
	q.tenantCol = e.TenantColumn
	return q
}

// Update starts UPDATE.
// SoftColumn on Entity enables soft-delete (does not update already soft-deleted rows).
func Update(e field.Entity) Query {
	q := Query{ast: expr.NewUpdate(e.Ref())}
	q.softCol = e.SoftColumn
	q.tenantCol = e.TenantColumn
	if e.SoftColumn != "" {
		q.soft = true
	}
	return q
}

// DeleteFrom starts DELETE.
// SoftColumn on Entity → soft delete (UPDATE deleted_at); use Hard() for physical DELETE.
func DeleteFrom(e field.Entity) Query {
	q := Query{ast: expr.NewDelete(e.Ref())}
	q.softCol = e.SoftColumn
	q.tenantCol = e.TenantColumn
	if e.SoftColumn != "" {
		q.soft = true
	}
	return q
}

// Values sets INSERT/UPSERT columns/values (one row).
func (q Query) Values(sets ...expr.Assignment) Query {
	q = q.clone()
	cols := make([]string, len(sets))
	vals := make([]any, len(sets))
	for i, s := range sets {
		cols[i] = s.Field.Column
		vals[i] = s.Value
	}
	q.ast.Columns = cols
	q.ast.Values = vals
	q.ast.MultiValues = nil
	return q
}

// Rows sets multi-row INSERT/UPSERT (bulk). Each row must have the same columns.
func (q Query) Rows(rows ...[]expr.Assignment) Query {
	q = q.clone()
	if len(rows) == 0 {
		return q
	}
	cols := make([]string, len(rows[0]))
	for i, s := range rows[0] {
		cols[i] = s.Field.Column
	}
	multi := make([][]any, len(rows))
	for i, row := range rows {
		vals := make([]any, len(row))
		for j, s := range row {
			vals[j] = s.Value
		}
		multi[i] = vals
	}
	q.ast.Columns = cols
	q.ast.Values = nil
	q.ast.MultiValues = multi
	return q
}

// Chunk sets the bulk-insert batch size (0 = all at once).
func (q Query) Chunk(n int) Query {
	q = q.clone()
	q.ast.ChunkSize = n
	return q
}

// OnConflict sets ON CONFLICT columns (upsert).
func (q Query) OnConflict(cols ...expr.FieldRef) Query {
	q = q.clone()
	if q.ast.Conflict == nil {
		q.ast.Conflict = &expr.ConflictClause{}
	}
	q.ast.Conflict.Columns = append([]expr.FieldRef(nil), cols...)
	if q.ast.Conflict.Action == 0 {
		q.ast.Conflict.Action = expr.ConflictDoUpdate
	}
	return q
}

// DoNothing sets ON CONFLICT DO NOTHING.
func (q Query) DoNothing() Query {
	q = q.clone()
	if q.ast.Conflict == nil {
		q.ast.Conflict = &expr.ConflictClause{}
	}
	q.ast.Conflict.Action = expr.ConflictDoNothing
	q.ast.Conflict.Updates = nil
	return q
}

// DoUpdate sets ON CONFLICT DO UPDATE SET …
func (q Query) DoUpdate(sets ...expr.Assignment) Query {
	q = q.clone()
	if q.ast.Conflict == nil {
		q.ast.Conflict = &expr.ConflictClause{}
	}
	q.ast.Conflict.Action = expr.ConflictDoUpdate
	q.ast.Conflict.Updates = append([]expr.Assignment(nil), sets...)
	return q
}

// Set sets UPDATE SET.
func (q Query) Set(sets ...expr.Assignment) Query {
	q = q.clone()
	q.ast.Sets = append([]expr.Assignment(nil), sets...)
	return q
}

// Returning sets RETURNING.
func (q Query) Returning(fields ...expr.FieldRef) Query {
	q = q.clone()
	q.ast.NoReturning = false
	q.ast.Returning = append([]expr.FieldRef(nil), fields...)
	return q
}

// OmitReturning skips RETURNING * (use for bulk UPDATE/INSERT when rows must not be scanned).
func (q Query) OmitReturning() Query {
	q = q.clone()
	q.ast.NoReturning = true
	q.ast.Returning = nil
	return q
}
