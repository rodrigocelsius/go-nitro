// Package expr defines the public BLinq AST (stable version).
package expr

import "time"

// ASTVersion increments on breaking AST changes.
const ASTVersion = 1

// Kind identifies the query type.
type Kind int

const (
	KindSelect Kind = iota + 1
	KindInsert
	KindUpdate
	KindDelete
	KindUpsert
	KindAggregate
)

// Op is a binary / unary operator in predicates.
type Op string

const (
	OpEq           Op = "="
	OpNeq          Op = "<>"
	OpGt           Op = ">"
	OpGte          Op = ">="
	OpLt           Op = "<"
	OpLte          Op = "<="
	OpLike         Op = "LIKE"
	OpILike        Op = "ILIKE"
	OpIn           Op = "IN"
	OpBetween      Op = "BETWEEN"
	OpIsNull       Op = "IS NULL"
	OpIsNotNull    Op = "IS NOT NULL"
	OpAnd          Op = "AND"
	OpOr           Op = "OR"
	OpNot          Op = "NOT"
	OpJSONContains Op = "@>"
	OpJSONPathEq   Op = "JSON_PATH_EQ"
)

// FieldRef references a column (no reflection).
type FieldRef struct {
	Table  string
	Column string
}

func (f FieldRef) Qualifier() string {
	if f.Table == "" {
		return f.Column
	}
	return f.Table + "." + f.Column
}

// Predicate is a filter tree node.
type Predicate interface {
	pred()
}

// Binary compares a field with a value (or IN/BETWEEN).
type Binary struct {
	Field FieldRef
	Op    Op
	Value any // scalar, []any for IN, [2]any for BETWEEN, or *Query / Subquery for IN (SELECT…)
}

func (Binary) pred() {}

// And conjugates predicates.
type And struct {
	Items []Predicate
}

func (And) pred() {}

// Or disjoins predicates.
type Or struct {
	Items []Predicate
}

func (Or) pred() {}

// Not negates a predicate.
type Not struct {
	Inner Predicate
}

func (Not) pred() {}

// Subquery wraps a nested SELECT (IN / EXISTS).
type Subquery struct {
	Query Query
}

// Exists is an EXISTS predicate (SELECT …).
// Correlation: use OuterRef in Sub's Where (e.g. child.FK = OuterRef{parent.ID}).
type Exists struct {
	Sub Query
}

func (Exists) pred() {}

// OuterRef references a column of the outer query (correlated subquery).
// Used as Binary.Value (e.g. EqOuter) — compiled as table.col with no placeholder.
type OuterRef struct {
	Field FieldRef
}

// RawPred is a raw SQL predicate (SQL provider embeds as (sql) + args).
type RawPred struct {
	SQL  string
	Args []any
}

func (RawPred) pred() {}

// Expr is a raw SQL expression (e.g. SET col = Expr{…}).
type Expr struct {
	SQL  string
	Args []any
}

// JSONValue is Binary's value for OpJSONContains / OpJSONPathEq.
type JSONValue struct {
	Path  string // used in OpJSONPathEq (extract path)
	Value any
}

// Include specifies batch-loading a relation (anti-N+1).
type Include struct {
	Name       string    // key on the parent map (e.g. "orders")
	Target     EntityRef // child table
	LocalKey   FieldRef  // PK / key on the parent
	ForeignKey FieldRef  // FK on the child
}

// OrderDir is sort direction.
type OrderDir string

const (
	OrderAsc  OrderDir = "ASC"
	OrderDesc OrderDir = "DESC"
)

// Order is ORDER BY.
type Order struct {
	Field FieldRef
	Dir   OrderDir
}

// AggFunc is a SQL aggregate function.
type AggFunc string

const (
	AggCount AggFunc = "COUNT"
	AggSum   AggFunc = "SUM"
	AggAvg   AggFunc = "AVG"
	AggMin   AggFunc = "MIN"
	AggMax   AggFunc = "MAX"
)

// Aggregate describes SELECT FUNC(col) [or COUNT(*)].
type Aggregate struct {
	Func  AggFunc
	Field FieldRef // empty + COUNT → COUNT(*)
}

// SelectItem is a column, *, or aggregate in the SELECT list.
type SelectItem struct {
	Field FieldRef
	All   bool
	Agg   *Aggregate
	Alias string
}

// SoftNow marks Assignment.Value for dialect SoftDeleteNow() (e.g. NOW()).
type SoftNow struct{}

// Inc marks Assignment.Value as col = col + delta.
type Inc struct {
	Delta any
}

// CacheHint is optional on the AST (the provider decides whether to honor it).
type CacheHint struct {
	TTL time.Duration
	Tag string
}

// JoinType is INNER or LEFT.
type JoinType string

const (
	JoinInner JoinType = "INNER"
	JoinLeft  JoinType = "LEFT"
)

// Join is JOIN table ON left = right.
type Join struct {
	Type  JoinType
	To    EntityRef
	Left  FieldRef
	Right FieldRef
}

// ConflictAction for Upsert.
type ConflictAction int

const (
	ConflictDoNothing ConflictAction = iota + 1
	ConflictDoUpdate
)

// ConflictClause is ON CONFLICT (...).
type ConflictClause struct {
	Columns []FieldRef
	Action  ConflictAction
	Updates []Assignment // DO UPDATE SET (empty = all EXCLUDED cols)
}

// EntityRef is a table/collection.
type EntityRef struct {
	Name   string
	Alias  string
	Schema string
}

// Query is the full AST.
type Query struct {
	Version  int
	Kind     Kind
	From     EntityRef
	Selects  []SelectItem
	Where    Predicate
	OrderBy  []Order
	Limit    *uint64
	Offset   *uint64
	Distinct bool

	Joins   []Join
	GroupBy []FieldRef
	Having  Predicate

	Sets        []Assignment
	Columns     []string
	Values      []any
	MultiValues [][]any
	ChunkSize   int
	Returning   []FieldRef
	// NoReturning skips RETURNING * on dialects that support it (bulk UPDATE/INSERT).
	NoReturning bool
	Conflict    *ConflictClause

	SoftDelete   bool
	SoftColumn   string
	TenantID     any
	TenantColumn string

	Cache *CacheHint

	Includes []Include

	Aggregate *Aggregate

	// Lock adds row locking to SELECT (queues, outbox, reservations).
	Lock *LockClause
}

// LockClause is SELECT … FOR UPDATE / FOR SHARE [SKIP LOCKED | NOWAIT].
type LockClause struct {
	Share      bool // FOR SHARE instead of FOR UPDATE
	SkipLocked bool
	NoWait     bool
}

// Assignment is SET col = value (UPDATE) or an INSERT column via Values.
type Assignment struct {
	Field FieldRef
	Value any // scalar, SoftNow{}, Inc{}, or Expr{}
}

// NewSelect creates a KindSelect Query with ASTVersion.
func NewSelect(from EntityRef) Query {
	return Query{
		Version: ASTVersion,
		Kind:    KindSelect,
		From:    from,
	}
}

// NewInsert creates a KindInsert Query.
func NewInsert(from EntityRef) Query {
	return Query{Version: ASTVersion, Kind: KindInsert, From: from}
}

// NewUpdate creates a KindUpdate Query.
func NewUpdate(from EntityRef) Query {
	return Query{Version: ASTVersion, Kind: KindUpdate, From: from}
}

// NewDelete creates a KindDelete Query.
func NewDelete(from EntityRef) Query {
	return Query{Version: ASTVersion, Kind: KindDelete, From: from}
}

// NewUpsert creates a KindUpsert Query.
func NewUpsert(from EntityRef) Query {
	return Query{Version: ASTVersion, Kind: KindUpsert, From: from}
}

// NewAggregate creates a KindAggregate Query.
func NewAggregate(from EntityRef, agg Aggregate) Query {
	return Query{
		Version:   ASTVersion,
		Kind:      KindAggregate,
		From:      from,
		Aggregate: &agg,
	}
}
