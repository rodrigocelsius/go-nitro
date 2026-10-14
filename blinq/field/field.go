// Package field provides typed column descriptors for the BLinq DSL.
package field

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

// Field is a typed column descriptor (generated or hand-written).
type Field[T any] struct {
	Table  string
	Column string
}

// Ref converts to expr.FieldRef.
func (f Field[T]) Ref() expr.FieldRef {
	return expr.FieldRef{Table: f.Table, Column: f.Column}
}

// Pred is a fluent predicate (chainable with And/Or).
type Pred struct {
	node expr.Predicate
}

func (p Pred) Node() expr.Predicate { return p.node }

// PredFrom wraps an AST predicate (Exists, Subquery, etc.).
func PredFrom(node expr.Predicate) Pred {
	return Pred{node: node}
}

func (p Pred) And(other Pred) Pred {
	return Pred{node: expr.And{Items: []expr.Predicate{p.node, other.node}}}
}

func (p Pred) Or(other Pred) Pred {
	return Pred{node: expr.Or{Items: []expr.Predicate{p.node, other.node}}}
}

func Not(p Pred) Pred {
	return Pred{node: expr.Not{Inner: p.node}}
}

func bin[T any](f Field[T], op expr.Op, v any) Pred {
	return Pred{node: expr.Binary{Field: f.Ref(), Op: op, Value: v}}
}

func (f Field[T]) Eq(v T) Pred  { return bin(f, expr.OpEq, v) }
func (f Field[T]) Neq(v T) Pred { return bin(f, expr.OpNeq, v) }

// EqOuter compares with a column of the outer query (correlated subquery).
func (f Field[T]) EqOuter(outer expr.FieldRef) Pred {
	return bin(f, expr.OpEq, expr.OuterRef{Field: outer})
}

// JSONContains builds a JSON @> predicate (Postgres) / subset on other dialects.
func (f Field[T]) JSONContains(v any) Pred {
	return bin(f, expr.OpJSONContains, expr.JSONValue{Value: v})
}

// JSONPathEq compares a value extracted from a JSON path with v.
func (f Field[T]) JSONPathEq(path string, v any) Pred {
	return bin(f, expr.OpJSONPathEq, expr.JSONValue{Path: path, Value: v})
}

// Raw wraps raw SQL as a predicate.
func Raw(sql string, args ...any) Pred {
	return PredFrom(expr.RawPred{SQL: sql, Args: args})
}

func (f Field[T]) Gt(v T) Pred     { return bin(f, expr.OpGt, v) }
func (f Field[T]) Gte(v T) Pred    { return bin(f, expr.OpGte, v) }
func (f Field[T]) Lt(v T) Pred     { return bin(f, expr.OpLt, v) }
func (f Field[T]) Lte(v T) Pred    { return bin(f, expr.OpLte, v) }
func (f Field[T]) IsNull() Pred    { return bin(f, expr.OpIsNull, nil) }
func (f Field[T]) IsNotNull() Pred { return bin(f, expr.OpIsNotNull, nil) }

func (f Field[T]) In(values ...T) Pred {
	anyVals := make([]any, len(values))
	for i, v := range values {
		anyVals[i] = v
	}
	return bin(f, expr.OpIn, anyVals)
}

func (f Field[T]) Between(a, b T) Pred {
	return bin(f, expr.OpBetween, []any{a, b})
}

// InRange is a half-open interval: field >= lo AND field < hiExclusive
// (useful for month buckets / date windows).
func (f Field[T]) InRange(lo, hiExclusive T) Pred {
	return f.Gte(lo).And(f.Lt(hiExclusive))
}

// EqValue compares a *T (nullable) column to a non-pointer value without
// forcing the caller to take the address of a temporary.
//
//	store.Query().Where(field.EqValue(Tracks.GenreID, id))
func EqValue[T any](f Field[*T], v T) Pred {
	vv := v
	return f.Eq(&vv)
}

// Text ops (Field typed as string in the app).
func (f Field[T]) Like(pattern string) Pred  { return bin(f, expr.OpLike, pattern) }
func (f Field[T]) ILike(pattern string) Pred { return bin(f, expr.OpILike, pattern) }

// escapeLikeText escapes the LIKE/ILIKE special characters ('\', '%', '_')
// in user-supplied text before it is wrapped in wildcards by
// Contains/IContains/StartsWith/EndsWith. Without this, a value like
// "50%_off" typed by an end user would silently behave as a wildcard
// pattern instead of a literal substring match ("LIKE injection" —
// behavioral, not SQL injection, since the value is still bound as a
// placeholder argument; see vuln.md V-34). The SQL compiler pairs this
// with an explicit "ESCAPE '\'" clause so '\' is recognized as the
// escape character on every dialect, including ones (SQLite, MSSQL)
// that don't default to it.
func escapeLikeText(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, "%", `\%`)
	text = strings.ReplaceAll(text, "_", `\_`)
	return text
}

func (f Field[T]) Contains(text string) Pred {
	return f.Like("%" + escapeLikeText(text) + "%")
}
func (f Field[T]) IContains(text string) Pred {
	return f.ILike("%" + escapeLikeText(text) + "%")
}
func (f Field[T]) StartsWith(prefix string) Pred {
	return f.Like(escapeLikeText(prefix) + "%")
}
func (f Field[T]) EndsWith(suffix string) Pred {
	return f.Like("%" + escapeLikeText(suffix))
}

// IsTrue / IsFalse for bool fields (typed Eq).
func IsTrue(f Field[bool]) Pred  { return f.Eq(true) }
func IsFalse(f Field[bool]) Pred { return f.Eq(false) }

// OrderAsc / OrderDesc.
func (f Field[T]) OrderAsc() expr.Order {
	return expr.Order{Field: f.Ref(), Dir: expr.OrderAsc}
}
func (f Field[T]) OrderDesc() expr.Order {
	return expr.Order{Field: f.Ref(), Dir: expr.OrderDesc}
}

// Set creates a typed UPDATE/INSERT Assignment.
func (f Field[T]) Set(v T) expr.Assignment {
	return expr.Assignment{Field: f.Ref(), Value: v}
}

// SetExpr creates an Assignment with a raw SQL expression (SET col = expr).
func (f Field[T]) SetExpr(e expr.Expr) expr.Assignment {
	return expr.Assignment{Field: f.Ref(), Value: e}
}

// Inc creates Assignment col = col + delta.
func (f Field[T]) Inc(delta T) expr.Assignment {
	return expr.Assignment{Field: f.Ref(), Value: expr.Inc{Delta: delta}}
}

// Entity is table metadata.
type Entity struct {
	Name   string
	Schema string
	Alias  string
	// SoftColumn on Entity enables soft-delete automatically (explicit Soft() is optional).
	SoftColumn string
	// TenantColumn, if non-empty, is the default for .Tenant() (e.g. "tenant_id").
	TenantColumn string
}

func (e Entity) Ref() expr.EntityRef {
	return expr.EntityRef{Name: e.Name, Schema: e.Schema, Alias: e.Alias}
}

// Col creates a Field (prototypes / hand-written metamodel).
func Col[T any](table, column string) Field[T] {
	return Field[T]{Table: table, Column: column}
}
