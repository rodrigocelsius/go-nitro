// Package client executes BLinq AST via the official go-elasticsearch client.
// Compile-only permanece em blinq/provider/elastic (zero deps).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/elastic"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// SearchAPI abstracts the Search API for tests with a fake transport.
type SearchAPI interface {
	Search(ctx context.Context, index string, body []byte) ([]map[string]any, error)
}

// Provider runs SELECT via Elasticsearch Search.
type Provider struct {
	API SearchAPI
}

var _ provider.Provider = (*Provider)(nil)

// New wraps *elasticsearch.Client.
func New(es *elasticsearch.Client) *Provider {
	if es == nil {
		return &Provider{}
	}
	return &Provider{API: &esSearch{es: es}}
}

// NewWithAPI allows injecting SearchAPI (tests).
func NewWithAPI(api SearchAPI) *Provider {
	return &Provider{API: api}
}

func (p *Provider) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	s, err := elastic.CompileSearch(q)
	if err != nil {
		return provider.Plan{}, err
	}
	b, err := json.Marshal(s.Body)
	if err != nil {
		return provider.Plan{}, err
	}
	return provider.Plan{Text: string(b)}, nil
}

func (p *Provider) Execute(ctx context.Context, q expr.Query, _ ...provider.Option) (provider.Result, error) {
	if p.API == nil {
		return nil, fmt.Errorf("blinq/elastic/client: API nil")
	}
	if q.Kind != expr.KindSelect && q.Kind != expr.KindAggregate {
		return nil, fmt.Errorf("blinq/elastic/client: only Select/Aggregate")
	}
	s, err := elastic.CompileSearch(q)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(s.Body)
	if err != nil {
		return nil, err
	}
	rows, err := p.API.Search(ctx, s.Index, body)
	if err != nil {
		return nil, err
	}
	return &mapResult{rows: rows}, nil
}

type esSearch struct {
	es *elasticsearch.Client
}

func (e *esSearch) Search(ctx context.Context, index string, body []byte) ([]map[string]any, error) {
	res, err := e.es.Search(
		e.es.Search.WithContext(ctx),
		e.es.Search.WithIndex(index),
		e.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("blinq/elastic/client: search %s: %s", res.Status(), string(b))
	}
	return parseHits(res.Body)
}

func parseHits(r io.Reader) ([]map[string]any, error) {
	var raw struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		src := h.Source
		if src == nil {
			src = map[string]any{}
		}
		out = append(out, src)
	}
	return out, nil
}

// ParseHitsJSON is useful for tests / fake transport.
func ParseHitsJSON(data []byte) ([]map[string]any, error) {
	return parseHits(bytes.NewReader(data))
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

func scanMaps(dest any, rows []map[string]any) error {
	if dest == nil {
		return fmt.Errorf("blinq/elastic/client: ScanSlice dest nil")
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("blinq/elastic/client: ScanSlice expects *[]T")
	}
	slice := rv.Elem()
	et := slice.Type().Elem()
	if et.Kind() == reflect.Map {
		out := reflect.MakeSlice(slice.Type(), len(rows), len(rows))
		for i, row := range rows {
			out.Index(i).Set(reflect.ValueOf(row))
		}
		slice.Set(out)
		return nil
	}
	out := reflect.MakeSlice(slice.Type(), 0, len(rows))
	for _, row := range rows {
		elem := reflect.New(et)
		if err := scan.MapToStruct(row, elem.Interface()); err != nil {
			return err
		}
		out = reflect.Append(out, elem.Elem())
	}
	slice.Set(out)
	return nil
}
