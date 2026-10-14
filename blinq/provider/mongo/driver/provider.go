// Package driver executes BLinq AST via the official mongo-driver.
// Compile-only permanece em blinq/provider/mongo (zero deps).
package driver

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	blmongo "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/mongo"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// Provider runs Select/Insert/Update/Delete on MongoDB.
type Provider struct {
	DB DatabaseAPI
}

var _ provider.Provider = (*Provider)(nil)

// New wraps *mongo.Database.
func New(db *mongo.Database) *Provider {
	if db == nil {
		return &Provider{}
	}
	return &Provider{DB: mongoDB{db}}
}

// NewWithDB allows injecting DatabaseAPI (tests / fakes).
func NewWithDB(db DatabaseAPI) *Provider {
	return &Provider{DB: db}
}

func (p *Provider) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	if len(q.Includes) > 0 {
		pipe, err := buildPipeline(q)
		if err != nil {
			return provider.Plan{}, err
		}
		b, err := json.Marshal(pipe)
		if err != nil {
			return provider.Plan{}, err
		}
		return provider.Plan{Text: string(b)}, nil
	}
	f, err := blmongo.CompileFind(stripIncludes(q))
	if err != nil {
		return provider.Plan{}, err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return provider.Plan{}, err
	}
	return provider.Plan{Text: string(b)}, nil
}

func (p *Provider) Execute(ctx context.Context, q expr.Query, _ ...provider.Option) (provider.Result, error) {
	if p.DB == nil {
		return nil, fmt.Errorf("blinq/mongo/driver: DB nil")
	}
	coll := p.DB.Collection(q.From.Name)

	switch q.Kind {
	case expr.KindSelect, expr.KindAggregate:
		return p.execSelect(ctx, coll, q)
	case expr.KindInsert:
		return p.execInsert(ctx, coll, q)
	case expr.KindUpdate:
		return p.execUpdate(ctx, coll, q)
	case expr.KindDelete:
		return p.execDelete(ctx, coll, q)
	default:
		return nil, fmt.Errorf("blinq/mongo/driver: unsupported kind %d", q.Kind)
	}
}

func (p *Provider) execSelect(ctx context.Context, coll CollectionAPI, q expr.Query) (provider.Result, error) {
	if len(q.Includes) > 0 {
		pipe, err := buildPipeline(q)
		if err != nil {
			return nil, err
		}
		cur, err := coll.Aggregate(ctx, pipe)
		if err != nil {
			return nil, err
		}
		var docs []bson.M
		if err := cur.All(ctx, &docs); err != nil {
			return nil, err
		}
		return &mapResult{rows: docsToMaps(docs)}, nil
	}

	f, err := blmongo.CompileFind(q)
	if err != nil {
		return nil, err
	}
	opts := options.Find()
	if f.Sort != nil {
		opts.SetSort(f.Sort)
	}
	if f.Limit != nil {
		opts.SetLimit(*f.Limit)
	}
	if f.Skip != nil {
		opts.SetSkip(*f.Skip)
	}
	cur, err := coll.Find(ctx, f.Filter, opts)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return &mapResult{rows: docsToMaps(docs)}, nil
}

func (p *Provider) execInsert(ctx context.Context, coll CollectionAPI, q expr.Query) (provider.Result, error) {
	if len(q.Columns) == 0 {
		return nil, fmt.Errorf("blinq/mongo/driver: insert columns required")
	}
	rows := q.MultiValues
	if len(rows) == 0 {
		rows = [][]any{q.Values}
	}
	var n int64
	for _, row := range rows {
		if len(row) != len(q.Columns) {
			return nil, fmt.Errorf("blinq/mongo/driver: columns/values mismatch")
		}
		doc := bson.M{}
		for i, c := range q.Columns {
			doc[c] = row[i]
		}
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			return nil, err
		}
		n++
	}
	return &writeResult{n: n}, nil
}

func (p *Provider) execUpdate(ctx context.Context, coll CollectionAPI, q expr.Query) (provider.Result, error) {
	filter := bson.M{}
	if q.Where != nil {
		f, err := blmongo.CompileFind(expr.Query{Kind: expr.KindSelect, From: q.From, Where: q.Where})
		if err != nil {
			return nil, err
		}
		filter = f.Filter
	}
	set := bson.M{}
	for _, a := range q.Sets {
		set[a.Field.Column] = a.Value
	}
	res, err := coll.UpdateMany(ctx, filter, bson.M{"$set": set})
	if err != nil {
		return nil, err
	}
	return &writeResult{n: res.ModifiedCount}, nil
}

func (p *Provider) execDelete(ctx context.Context, coll CollectionAPI, q expr.Query) (provider.Result, error) {
	filter := bson.M{}
	if q.Where != nil {
		f, err := blmongo.CompileFind(expr.Query{Kind: expr.KindSelect, From: q.From, Where: q.Where})
		if err != nil {
			return nil, err
		}
		filter = f.Filter
	}
	res, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &writeResult{n: res.DeletedCount}, nil
}

func buildPipeline(q expr.Query) ([]bson.M, error) {
	base := stripIncludes(q)
	f, err := blmongo.CompileFind(base)
	if err != nil {
		return nil, err
	}
	pipe := make([]bson.M, 0, 2+len(q.Includes))
	if len(f.Filter) > 0 {
		pipe = append(pipe, bson.M{"$match": f.Filter})
	}
	for _, inc := range q.Includes {
		stage := blmongo.CompileLookup(inc)
		m := bson.M{}
		for k, v := range stage {
			m[k] = v
		}
		pipe = append(pipe, m)
	}
	if f.Sort != nil {
		pipe = append(pipe, bson.M{"$sort": f.Sort})
	}
	if f.Skip != nil {
		pipe = append(pipe, bson.M{"$skip": *f.Skip})
	}
	if f.Limit != nil {
		pipe = append(pipe, bson.M{"$limit": *f.Limit})
	}
	return pipe, nil
}

func stripIncludes(q expr.Query) expr.Query {
	q.Includes = nil
	return q
}

type mapResult struct {
	rows []map[string]any
}

func (r *mapResult) Err() error            { return nil }
func (r *mapResult) Count() (int64, error) { return int64(len(r.rows)), nil }
func (r *mapResult) ScanSlice(dest any) error {
	return scanMaps(dest, r.rows)
}
func (r *mapResult) ScanOne(dest any) error {
	if len(r.rows) == 0 {
		return provider.ErrNotFound
	}
	return scan.MapToStruct(r.rows[0], dest)
}

type writeResult struct{ n int64 }

func (r *writeResult) Err() error               { return nil }
func (r *writeResult) Count() (int64, error)    { return r.n, nil }
func (r *writeResult) ScanSlice(dest any) error { return provider.ErrUnsupported }
func (r *writeResult) ScanOne(dest any) error   { return provider.ErrUnsupported }
