package driver

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CursorAPI abstracts cursor.All for offline tests.
type CursorAPI interface {
	All(ctx context.Context, results any) error
}

// CollectionAPI abstracts collection operations (Find/Aggregate/Insert/Update/Delete).
type CollectionAPI interface {
	Find(ctx context.Context, filter any, opts ...*options.FindOptions) (CursorAPI, error)
	Aggregate(ctx context.Context, pipeline any, opts ...*options.AggregateOptions) (CursorAPI, error)
	InsertOne(ctx context.Context, document any, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error)
	UpdateMany(ctx context.Context, filter any, update any, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error)
	DeleteMany(ctx context.Context, filter any, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error)
}

// DatabaseAPI resolves collections by name.
type DatabaseAPI interface {
	Collection(name string) CollectionAPI
}

type mongoCursor struct {
	*mongo.Cursor
}

func (c mongoCursor) All(ctx context.Context, results any) error {
	return c.Cursor.All(ctx, results)
}

type mongoColl struct {
	*mongo.Collection
}

func (c mongoColl) Find(ctx context.Context, filter any, opts ...*options.FindOptions) (CursorAPI, error) {
	cur, err := c.Collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	return mongoCursor{cur}, nil
}

func (c mongoColl) Aggregate(ctx context.Context, pipeline any, opts ...*options.AggregateOptions) (CursorAPI, error) {
	cur, err := c.Collection.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, err
	}
	return mongoCursor{cur}, nil
}

func (c mongoColl) InsertOne(ctx context.Context, document any, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	return c.Collection.InsertOne(ctx, document, opts...)
}

func (c mongoColl) UpdateMany(ctx context.Context, filter any, update any, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return c.Collection.UpdateMany(ctx, filter, update, opts...)
}

func (c mongoColl) DeleteMany(ctx context.Context, filter any, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	return c.Collection.DeleteMany(ctx, filter, opts...)
}

type mongoDB struct {
	*mongo.Database
}

func (d mongoDB) Collection(name string) CollectionAPI {
	return mongoColl{d.Database.Collection(name)}
}

// docsToMaps converts []bson.M → []map[string]any.
func docsToMaps(docs []bson.M) []map[string]any {
	out := make([]map[string]any, len(docs))
	for i, d := range docs {
		m := make(map[string]any, len(d))
		for k, v := range d {
			m[k] = v
		}
		out[i] = m
	}
	return out
}
