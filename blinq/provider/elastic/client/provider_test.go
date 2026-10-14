package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/rodrigocelsius/go-nitro/blinq/provider/elastic/client/v2"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type fakeAPI struct {
	lastIndex string
	lastBody  []byte
	rows      []map[string]any
}

func (f *fakeAPI) Search(_ context.Context, index string, body []byte) ([]map[string]any, error) {
	f.lastIndex = index
	f.lastBody = append([]byte(nil), body...)
	return f.rows, nil
}

func TestExecuteSelect(t *testing.T) {
	api := &fakeAPI{rows: []map[string]any{{"name": "Ada", "age": 30}}}
	p := client.NewWithAPI(api)
	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Gte(18)).Limit(10)
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
	if api.lastIndex != "users" {
		t.Fatal(api.lastIndex)
	}
	var body map[string]any
	if err := json.Unmarshal(api.lastBody, &body); err != nil {
		t.Fatal(err)
	}
	if body["size"] != float64(10) {
		t.Fatalf("%#v", body)
	}
}

func TestParseHitsJSON(t *testing.T) {
	raw := []byte(`{"hits":{"hits":[{"_source":{"name":"Ada"}}]}}`)
	rows, err := client.ParseHitsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Ada" {
		t.Fatalf("%#v", rows)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNewWithFakeTransport(t *testing.T) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := `{"hits":{"hits":[{"_source":{"id":1}}]}}`
			return &http.Response{
				StatusCode: 200,
				Body:       ioNopCloser(strings.NewReader(body)),
				Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
			}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	p := client.New(es)
	res, err := p.Execute(context.Background(), query.FromName("users").AST())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := res.Count()
	if n != 1 {
		t.Fatal(n)
	}
}

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

func ioNopCloser(r *strings.Reader) *nopCloser { return &nopCloser{r} }
