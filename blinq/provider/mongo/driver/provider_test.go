package driver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/rodrigocelsius/go-nitro/blinq/provider/mongo/driver/v2"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type fakeCursor struct {
	docs []bson.M
}

func (c *fakeCursor) All(_ context.Context, results any) error {
	ptr, ok := results.(*[]bson.M)
	if !ok {
		return nil
	}
	*ptr = append([]bson.M(nil), c.docs...)
	return nil
}

type fakeColl struct {
	name       string
	lastFilter any
	lastOpts   *options.FindOptions
	lastPipe   any
	findDocs   []bson.M
	aggDocs    []bson.M
	inserted   []any
	updated    int64
	deleted    int64
}

func (c *fakeColl) Find(_ context.Context, filter any, opts ...*options.FindOptions) (driver.CursorAPI, error) {
	c.lastFilter = filter
	if len(opts) > 0 {
		c.lastOpts = opts[0]
	}
	return &fakeCursor{docs: c.findDocs}, nil
}

func (c *fakeColl) Aggregate(_ context.Context, pipeline any, _ ...*options.AggregateOptions) (driver.CursorAPI, error) {
	c.lastPipe = pipeline
	return &fakeCursor{docs: c.aggDocs}, nil
}

func (c *fakeColl) InsertOne(_ context.Context, document any, _ ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	c.inserted = append(c.inserted, document)
	return &mongo.InsertOneResult{}, nil
}

func (c *fakeColl) UpdateMany(_ context.Context, _ any, _ any, _ ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return &mongo.UpdateResult{ModifiedCount: c.updated}, nil
}

func (c *fakeColl) DeleteMany(_ context.Context, _ any, _ ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	return &mongo.DeleteResult{DeletedCount: c.deleted}, nil
}

type fakeDB struct {
	colls map[string]*fakeColl
}

func (d *fakeDB) Collection(name string) driver.CollectionAPI {
	if c, ok := d.colls[name]; ok {
		return c
	}
	c := &fakeColl{name: name}
	d.colls[name] = c
	return c
}

func TestNewNil(t *testing.T) {
	p := driver.New(nil)
	if p.DB != nil {
		t.Fatal("expected nil DB")
	}
}

func TestExecuteFindRecordsFilter(t *testing.T) {
	coll := &fakeColl{
		findDocs: []bson.M{{"name": "Ada", "age": 30}},
	}
	db := &fakeDB{colls: map[string]*fakeColl{"users": coll}}
	p := driver.NewWithDB(db)

	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Gte(18)).Limit(5)
	res, err := p.Execute(context.Background(), q.AST())
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := res.ScanSlice(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["name"] != "Ada" {
		t.Fatalf("%#v", out)
	}
	filt, _ := coll.lastFilter.(map[string]any)
	if filt == nil {
		// bson.M is map[string]any alias in practice for CompileFind
		fm, ok := coll.lastFilter.(bson.M)
		if !ok {
			// CompileFind returns map[string]any
			m, ok := coll.lastFilter.(map[string]any)
			if !ok || m["age"] == nil {
				t.Fatalf("filter=%#v", coll.lastFilter)
			}
		} else if fm["age"] == nil {
			t.Fatalf("filter=%#v", coll.lastFilter)
		}
	} else if filt["age"] == nil {
		t.Fatalf("filter=%#v", coll.lastFilter)
	}
	if coll.lastOpts == nil || coll.lastOpts.Limit == nil || *coll.lastOpts.Limit != 5 {
		t.Fatalf("opts=%#v", coll.lastOpts)
	}
}

func TestExecuteAggregateWithInclude(t *testing.T) {
	coll := &fakeColl{
		aggDocs: []bson.M{{"name": "Ada", "orders": []any{}}},
	}
	db := &fakeDB{colls: map[string]*fakeColl{"users": coll}}
	p := driver.NewWithDB(db)

	rel := field.HasMany("orders", field.Entity{Name: "orders"},
		field.Col[int]("users", "id").Ref(),
		field.Col[int]("orders", "user_id").Ref(),
	)
	q := query.FromName("users").Include(rel)
	res, err := p.Execute(context.Background(), q.AST())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := res.Count()
	if n != 1 {
		t.Fatal(n)
	}
	pipe, ok := coll.lastPipe.([]bson.M)
	if !ok || len(pipe) == 0 {
		t.Fatalf("pipe=%#v", coll.lastPipe)
	}
	found := false
	for _, stage := range pipe {
		if stage["$lookup"] != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected $lookup in %#v", pipe)
	}
}

func TestExplainJSON(t *testing.T) {
	p := driver.NewWithDB(&fakeDB{colls: map[string]*fakeColl{}})
	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Eq(1))
	plan, err := p.Explain(context.Background(), q.AST())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Text, `"age"`) {
		t.Fatal(plan.Text)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(plan.Text), &m); err != nil {
		t.Fatal(err)
	}
}

func TestInsertRecords(t *testing.T) {
	coll := &fakeColl{}
	db := &fakeDB{colls: map[string]*fakeColl{"users": coll}}
	p := driver.NewWithDB(db)
	name := field.Col[string]("users", "name")
	q := query.InsertInto(field.Entity{Name: "users"}).Values(name.Set("Ada"))
	res, err := p.Execute(context.Background(), q.AST())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := res.Count()
	if n != 1 || len(coll.inserted) != 1 {
		t.Fatalf("n=%d inserted=%#v", n, coll.inserted)
	}
}
