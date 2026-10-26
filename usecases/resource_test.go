package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

// --- Mock ResourceRepository for testing ---

type mockResourceRepo struct {
	items     []map[string]any
	nextID    int
	findErr   error
	listErr   error
	createErr error
	updateErr error
	deleteErr error

	// found, when set, is returned by FindByID as-is (typed entity), taking
	// priority over the map-based `items` lookup used by other tests.
	found any
	// created/updated capture the exact entity passed to Create/Update, so
	// tests can assert on it after the handler runs (e.g. mass-assignment
	// checks on readonly fields).
	created any
	updated any
}

func newMockRepo() *mockResourceRepo {
	return &mockResourceRepo{
		items:  make([]map[string]any, 0),
		nextID: 1,
	}
}

func (r *mockResourceRepo) FindByID(_ context.Context, id any) (any, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.found != nil {
		return r.found, nil
	}
	for _, item := range r.items {
		if fmt.Sprintf("%v", item["id"]) == fmt.Sprintf("%v", id) {
			return item, nil
		}
	}
	return nil, nil
}

func (r *mockResourceRepo) List(_ context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	total := len(r.items)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	items := make([]any, len(r.items[start:end]))
	for i, item := range r.items[start:end] {
		items[i] = item
	}

	return &domain.ResourceListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: (total + perPage - 1) / perPage,
	}, nil
}

func (r *mockResourceRepo) Create(_ context.Context, entity any) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = entity
	r.nextID++
	return nil
}

func (r *mockResourceRepo) Update(_ context.Context, entity any) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updated = entity
	return nil
}

func (r *mockResourceRepo) Delete(_ context.Context, id any) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	return nil
}

type mockBinder struct {
	body []byte
}

func (m *mockBinder) Bind(v any) error {
	return json.Unmarshal(m.body, v)
}

func (m *mockBinder) Body() []byte {
	return m.body
}

type mockJSONResponder struct {
	mockBinder
	status int
	body   any
}

func (m *mockJSONResponder) JSON(status int, v any) error {
	m.status = status
	m.body = v
	return nil
}

func TestMapRepoError_PropagatesHTTPError(t *testing.T) {
	nf := domain.NewNotFound("todo not found")
	got := mapRepoError(nf, "fallback")
	if got != nf {
		t.Fatalf("expected same HTTPError, got %v", got)
	}

	he, ok := got.(domain.HTTPError)
	if !ok || he.StatusCode() != 404 {
		t.Fatalf("expected 404 HTTPError, got %T %v", got, got)
	}
}

func TestMapRepoError_NotFoundMessage(t *testing.T) {
	got := mapRepoError(domain.ErrNotFound, "users: not found")
	he, ok := got.(domain.HTTPError)
	if !ok || he.StatusCode() != 404 {
		t.Fatalf("expected 404, got %T %v", got, got)
	}
}

func TestMapRepoError_SubstringNotFoundIsInternal(t *testing.T) {
	// Substring matching removed — apps must use ErrNotFound / sql.ErrNoRows / NotFound().
	got := mapRepoError(fmt.Errorf("record not found"), "users: not found")
	he, ok := got.(domain.HTTPError)
	if !ok || he.StatusCode() != 500 {
		t.Fatalf("expected 500 for plain string, got %T %v", got, got)
	}
}

func TestMapRepoError_Internal(t *testing.T) {
	got := mapRepoError(fmt.Errorf("db down"), "")
	he, ok := got.(domain.HTTPError)
	if !ok || he.StatusCode() != 500 {
		t.Fatalf("expected 500, got %T %v", got, got)
	}
}

func TestCreateHandler_ValidatesTags(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required,min=2"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	err := create(ctx)
	ve, ok := err.(*domain.ValidationFailedError)
	if !ok {
		t.Fatalf("expected ValidationFailedError, got %T %v", err, err)
	}
	if ve.StatusCode() != 422 {
		t.Fatalf("status = %d", ve.StatusCode())
	}
	if _, ok := ve.Errors.Map()["name"]; !ok {
		t.Fatalf("fields = %v", ve.Errors.Map())
	}
}

func TestCreateHandler_InvalidJSON(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	err := create(ctx)
	he, ok := err.(domain.HTTPError)
	if !ok || he.StatusCode() != 400 {
		t.Fatalf("expected 400, got %T %v", err, err)
	}
}

func TestCreateHandler_Success(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required,min=2"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"ok"}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := create(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.status != 201 {
		t.Fatalf("status = %d, want 201", adapter.status)
	}
}

func TestCreateHandler_ReadonlyFieldBlocksMassAssignment(t *testing.T) {
	type Item struct {
		Name    string `json:"name" validate:"required,min=2"`
		IsAdmin bool   `json:"is_admin" readonly:"true"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"ok","is_admin":true}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := create(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	created, ok := repo.created.(*Item)
	if !ok {
		t.Fatalf("expected *Item, got %T", repo.created)
	}
	if created.IsAdmin {
		t.Fatal("readonly field IsAdmin must not be settable from the request body")
	}
}

func TestCreateHandler_ComputedFieldBlocksMassAssignment(t *testing.T) {
	type Item struct {
		Name       string `json:"name" validate:"required,min=2"`
		ArtistName string `json:"artist_name" computed:"true"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo, ResourceOpts{
		BeforeWrite: func(_ context.Context, entity any) error {
			it := entity.(*Item)
			it.ArtistName = "from-hook"
			return nil
		},
	})
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"ok","artist_name":"spoofed"}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := create(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	created := repo.created.(*Item)
	if created.ArtistName != "from-hook" {
		t.Fatalf("ArtistName = %q, want from-hook (computed cleared then BeforeWrite)", created.ArtistName)
	}
}

func TestCreateHandler_BeforeWriteRuns(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required,min=2"`
		Sort string `json:"sort"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo, ResourceOpts{
		BeforeWrite: func(_ context.Context, entity any) error {
			it := entity.(*Item)
			it.Sort = strings.ToLower(it.Name)
			return nil
		},
	})
	create := routes[2].Handler
	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"OK"}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := create(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created.(*Item).Sort != "ok" {
		t.Fatalf("Sort = %q, want ok", repo.created.(*Item).Sort)
	}
}

func TestUpdateHandler_ReadonlyFieldRestoredOnPatchMerge(t *testing.T) {
	type Item struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		IsAdmin bool   `json:"is_admin" readonly:"true"`
	}
	repo := newMockRepo()
	repo.found = &Item{ID: "1", Name: "old", IsAdmin: false}
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	update := routes[3].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"new","is_admin":true}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	ctx.ParamNames[0] = "id"
	ctx.ParamValues[0] = "1"
	ctx.ParamCount = 1

	if err := update(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated, ok := repo.updated.(*Item)
	if !ok {
		t.Fatalf("expected *Item, got %T", repo.updated)
	}
	if updated.IsAdmin {
		t.Fatal("readonly field IsAdmin must be restored to its pre-update value on PATCH merge")
	}
	if updated.Name != "new" {
		t.Fatalf("Name = %q, want new (non-readonly fields still merge)", updated.Name)
	}
}

func TestGetHandler_PropagatesNotFound(t *testing.T) {
	repo := newMockRepo()
	repo.findErr = domain.NewNotFound("todo not found")
	routes := GenerateResourceRoutes("/users", reflect.TypeOf(User{}), repo)
	get := routes[1].Handler

	ctx := &domain.Ctx{AdapterData: &mockJSONResponder{}}
	ctx.ParamNames[0] = "id"
	ctx.ParamValues[0] = "1"
	ctx.ParamCount = 1

	err := get(ctx)
	he, ok := err.(domain.HTTPError)
	if !ok || he.StatusCode() != 404 {
		t.Fatalf("expected 404 from repo HTTPError, got %T %v", err, err)
	}
	if err.Error() != "todo not found" {
		t.Fatalf("message = %q, want todo not found (propagated)", err.Error())
	}
}

func TestUpdateHandler_PropagatesNotFound(t *testing.T) {
	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name" validate:"required"`
	}
	repo := newMockRepo()
	repo.updateErr = domain.NewNotFound("todo not found")
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	update := routes[3].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"x"}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	ctx.ParamNames[0] = "id"
	ctx.ParamValues[0] = "99"
	ctx.ParamCount = 1

	err := update(ctx)
	he, ok := err.(domain.HTTPError)
	if !ok || he.StatusCode() != 404 {
		t.Fatalf("expected 404, got %T %v", err, err)
	}
}

func TestCreateHandler_AutoIDAndTimestamps(t *testing.T) {
	type Item struct {
		ID        string    `json:"id"`
		Name      string    `json:"name" validate:"required"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo)
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"ok"}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := create(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	item, ok := adapter.body.(*Item)
	if !ok {
		// body might be the pointer from reflect.New
		item, ok = adapter.body.(*Item)
		if !ok {
			// Check via map or type
			t.Fatalf("body type %T", adapter.body)
		}
	}
	if item.ID == "" {
		t.Fatal("expected auto ID")
	}
	if _, err := nitrouuid.Parse(item.ID); err != nil {
		t.Fatalf("ID should be UUID: %v", err)
	}
	if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps, got created=%v updated=%v", item.CreatedAt, item.UpdatedAt)
	}
}

func TestCreateHandler_DisableAutoID(t *testing.T) {
	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name" validate:"required"`
	}
	repo := newMockRepo()
	opts := ResourceOpts{AutoID: false, AutoTimestamps: false}
	routes := GenerateResourceRoutes("/items", reflect.TypeOf(Item{}), repo, opts)
	create := routes[2].Handler

	adapter := &mockJSONResponder{mockBinder: mockBinder{body: []byte(`{"name":"ok"}`)}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := create(ctx); err != nil {
		t.Fatal(err)
	}
	item := adapter.body.(*Item)
	if item.ID != "" {
		t.Fatalf("expected empty ID with AutoID disabled, got %q", item.ID)
	}
}

func TestApplyResourceDefaults(t *testing.T) {
	type Item struct {
		ID        string
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	item := &Item{}
	applyResourceDefaults(item, DefaultResourceOpts(), true)
	if item.ID == "" || item.CreatedAt.IsZero() {
		t.Fatalf("defaults not applied: %+v", item)
	}
	old := item.UpdatedAt
	time.Sleep(time.Millisecond)
	applyResourceDefaults(item, DefaultResourceOpts(), false)
	if item.UpdatedAt.Before(old) {
		t.Fatalf("UpdatedAt went backwards: %v < %v", item.UpdatedAt, old)
	}
	if item.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set on update")
	}
}

// --- Test Models ---

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type OrderItem struct {
	ID       string  `json:"id"`
	Product  string  `json:"product"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type HTTPResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// --- Test deriveTableName ---

func TestDeriveTableName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User", "users"},
		{"Order", "orders"},
		{"OrderItem", "order_items"},
		{"HTTPResponse", "http_responses"},
		{"Person", "people"},
		{"Category", "categories"},
		{"Bus", "buses"},
		{"Wolf", "wolves"},
		{"Knife", "knives"},
		{"City", "cities"},
		{"Boy", "boys"},
		{"Quiz", "quizzes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := deriveTableName(tt.input)
			if got != tt.want {
				t.Errorf("deriveTableName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Test ExtractResourceMetadata ---

func TestExtractResourceMetadata_User(t *testing.T) {
	meta := ExtractResourceMetadata(reflect.TypeOf(User{}))

	if meta.TableName != "users" {
		t.Errorf("TableName = %q, want %q", meta.TableName, "users")
	}
	if meta.IDColumn != "id" {
		t.Errorf("IDColumn = %q, want %q", meta.IDColumn, "id")
	}
	if len(meta.Fields) != 3 {
		t.Errorf("Fields count = %d, want 3", len(meta.Fields))
	}

	fieldMap := make(map[string]ResourceField)
	for _, f := range meta.Fields {
		fieldMap[f.JSONName] = f
	}

	if !fieldMap["id"].IsID {
		t.Error("field 'id' should be marked as ID")
	}
	if fieldMap["name"].Type.Kind() != reflect.String {
		t.Errorf("field 'name' type = %v, want string", fieldMap["name"].Type.Kind())
	}
}

func TestExtractResourceMetadata_PointerType(t *testing.T) {
	meta := ExtractResourceMetadata(reflect.TypeOf(&User{}))

	if meta.TableName != "users" {
		t.Errorf("TableName = %q, want %q (should handle pointer)", meta.TableName, "users")
	}
}

func TestExtractResourceMetadata_OrderItem(t *testing.T) {
	meta := ExtractResourceMetadata(reflect.TypeOf(OrderItem{}))

	if meta.TableName != "order_items" {
		t.Errorf("TableName = %q, want %q", meta.TableName, "order_items")
	}

	fieldMap := make(map[string]ResourceField)
	for _, f := range meta.Fields {
		fieldMap[f.JSONName] = f
	}

	if !fieldMap["id"].IsID {
		t.Error("field 'id' should be marked as ID")
	}
	if fieldMap["quantity"].Type.Kind() != reflect.Int {
		t.Errorf("field 'quantity' type = %v, want int", fieldMap["quantity"].Type.Kind())
	}
	if fieldMap["price"].Type.Kind() != reflect.Float64 {
		t.Errorf("field 'price' type = %v, want float64", fieldMap["price"].Type.Kind())
	}
}

// --- Test Pluralize ---

func TestPluralize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user", "users"},
		{"order", "orders"},
		{"box", "boxes"},
		{"wish", "wishes"},
		{"city", "cities"},
		{"boy", "boys"},
		{"wolf", "wolves"},
		{"knife", "knives"},
		{"cat", "cats"},
		{"dog", "dogs"},
		{"bus", "buses"},
		{"quiz", "quizzes"},
		{"person", "people"},
		{"man", "men"},
		{"child", "children"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Pluralize(tt.input)
			if got != tt.want {
				t.Errorf("Pluralize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Test toSnakeCase ---

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User", "user"},
		{"UserName", "user_name"},
		{"HTTPResponse", "http_response"},
		{"ID", "id"},
		{"CreatedAt", "created_at"},
		{"URL", "url"},
		{"Name", "name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Test GenerateResourceRoutes ---

func TestGenerateResourceRoutes_Count(t *testing.T) {
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/users", reflect.TypeOf(User{}), repo)

	if len(routes) != 5 {
		t.Errorf("routes count = %d, want 5", len(routes))
	}

	expected := []struct {
		method string
		path   string
	}{
		{"GET", "/users"},
		{"GET", "/users/:id"},
		{"POST", "/users"},
		{"PATCH", "/users/:id"},
		{"DELETE", "/users/:id"},
	}

	for i, exp := range expected {
		if routes[i].Method != exp.method {
			t.Errorf("route[%d].Method = %q, want %q", i, routes[i].Method, exp.method)
		}
		if routes[i].Path != exp.path {
			t.Errorf("route[%d].Path = %q, want %q", i, routes[i].Path, exp.path)
		}
	}
}

func TestGenerateResourceRoutes_DifferentModel(t *testing.T) {
	repo := newMockRepo()
	routes := GenerateResourceRoutes("/order-items", reflect.TypeOf(OrderItem{}), repo)

	if len(routes) != 5 {
		t.Errorf("routes count = %d, want 5", len(routes))
	}

	if routes[0].Path != "/order-items" {
		t.Errorf("list path = %q, want /order-items", routes[0].Path)
	}
}

// --- Test ScanResource (legacy controller-based) ---

type LegacyController struct{}

func (c *LegacyController) List(_ *domain.Ctx) error   { return nil }
func (c *LegacyController) Get(_ *domain.Ctx) error    { return nil }
func (c *LegacyController) Create(_ *domain.Ctx) error { return nil }
func (c *LegacyController) Update(_ *domain.Ctx) error { return nil }
func (c *LegacyController) Delete(_ *domain.Ctx) error { return nil }

func TestScanResource_Legacy(t *testing.T) {
	controller := &LegacyController{}
	routes := ScanResource("/items", controller)

	if len(routes) != 5 {
		t.Errorf("routes count = %d, want 5", len(routes))
	}

	methods := make(map[string]bool)
	for _, r := range routes {
		methods[r.Method] = true
	}

	if !methods["GET"] || !methods["POST"] || !methods["PATCH"] || !methods["DELETE"] {
		t.Errorf("missing expected HTTP methods, got: %v", methods)
	}
}

// --- Benchmarks ---

func BenchmarkExtractResourceMetadata(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ExtractResourceMetadata(reflect.TypeOf(User{}))
	}
}

func BenchmarkGenerateResourceRoutes(b *testing.B) {
	repo := newMockRepo()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		GenerateResourceRoutes("/users", reflect.TypeOf(User{}), repo)
	}
}
