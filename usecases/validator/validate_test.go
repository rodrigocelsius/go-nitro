package validator

import (
	"fmt"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

func TestValidateStruct_Min(t *testing.T) {
	type User struct {
		Name string `validate:"min=3"`
	}

	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{"valid", User{Name: "John"}, false},
		{"too short", User{Name: "Jo"}, true},
		{"exact", User{Name: "Bob"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.user)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Max(t *testing.T) {
	type User struct {
		Name string `validate:"max=5"`
	}

	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{"valid", User{Name: "John"}, false},
		{"too long", User{Name: "Johnny"}, true},
		{"exact", User{Name: "Johny"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.user)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Len(t *testing.T) {
	type Code struct {
		Value string `validate:"len=6"`
	}

	tests := []struct {
		name    string
		code    Code
		wantErr bool
	}{
		{"valid", Code{Value: "ABC123"}, false},
		{"too short", Code{Value: "ABC"}, true},
		{"too long", Code{Value: "ABC1234"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.code)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_In(t *testing.T) {
	type Status struct {
		Value string `validate:"in=active inactive pending"`
	}

	tests := []struct {
		name    string
		status  Status
		wantErr bool
	}{
		{"valid active", Status{Value: "active"}, false},
		{"valid inactive", Status{Value: "inactive"}, false},
		{"invalid", Status{Value: "deleted"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.status)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_NotIn(t *testing.T) {
	type Username struct {
		Value string `validate:"not_in=admin root system"`
	}

	tests := []struct {
		name     string
		username Username
		wantErr  bool
	}{
		{"valid", Username{Value: "john"}, false},
		{"reserved admin", Username{Value: "admin"}, true},
		{"reserved root", Username{Value: "root"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.username)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_MinSlice(t *testing.T) {
	type Tags struct {
		Items []string `validate:"min=2"`
	}

	tests := []struct {
		name    string
		tags    Tags
		wantErr bool
	}{
		{"valid", Tags{Items: []string{"a", "b"}}, false},
		{"too few", Tags{Items: []string{"a"}}, true},
		{"empty", Tags{Items: []string{}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.tags)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Combined(t *testing.T) {
	type User struct {
		Name   string `validate:"required,min=3,max=50"`
		Email  string `validate:"required,email"`
		Status string `validate:"required,in=active inactive"`
	}

	errs := ValidateStruct(User{})
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors for zero-value User, got %d", len(errs))
	}
}

func BenchmarkValidateStruct(b *testing.B) {
	// Pre-cache baseline (~Jul/2026, AMD Ryzen 7 8700G): ~1470 ns/op, 696 B/op, 29 allocs/op.
	// Phase 8 goal: do not regress >10% vs this baseline after type-meta cache.
	// Ver docs/extra/benchmarks.md § Validation.
	type User struct {
		Name   string `validate:"required,min=3,max=50"`
		Email  string `validate:"required,email"`
		Status string `validate:"required,in=active inactive"`
	}

	user := User{Name: "John", Email: "john@example.com", Status: "active"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateStruct(user)
	}
}

// --- gt/gte/lt/lte tests ---

func TestValidateStruct_Gt(t *testing.T) {
	type Item struct {
		Price float64 `validate:"gt=0"`
	}

	tests := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{"valid positive", Item{Price: 9.99}, false},
		{"zero", Item{Price: 0}, true},
		{"negative", Item{Price: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.item)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Gte(t *testing.T) {
	type Item struct {
		Quantity int `validate:"gte=0"`
	}

	tests := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{"valid zero", Item{Quantity: 0}, false},
		{"valid positive", Item{Quantity: 5}, false},
		{"negative", Item{Quantity: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.item)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Lt(t *testing.T) {
	type Item struct {
		Age int `validate:"lt=150"`
	}

	tests := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{"valid", Item{Age: 25}, false},
		{"boundary", Item{Age: 149}, false},
		{"at limit", Item{Age: 150}, true},
		{"over limit", Item{Age: 200}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.item)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Lte(t *testing.T) {
	type Item struct {
		Score float64 `validate:"lte=100"`
	}

	tests := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{"valid", Item{Score: 75.5}, false},
		{"at limit", Item{Score: 100}, false},
		{"over limit", Item{Score: 100.1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.item)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_GtInt(t *testing.T) {
	type Config struct {
		Workers int `validate:"gt=0"`
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Workers: 4}, false},
		{"zero", Config{Workers: 0}, true},
		{"negative", Config{Workers: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(tt.cfg)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateStruct() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_Email(t *testing.T) {
	type User struct {
		Email string `json:"email" validate:"email"`
	}

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid", "a@b.co", false},
		{"valid longer", "john@example.com", false},
		{"empty", "", true},
		{"only at", "@.", true},
		{"double at", "@@.", true},
		{"no local", "@domain.com", true},
		{"no domain dot", "a@b", true},
		{"trailing dot", "a@b.", true},
		{"space", "a @b.co", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateStruct(User{Email: tt.email})
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("email %q errors = %v, wantErr %v", tt.email, errs, tt.wantErr)
			}
		})
	}
}

func TestValidateStruct_JSONFieldNames(t *testing.T) {
	type User struct {
		Email string `json:"email" validate:"required,email"`
	}

	errs := ValidateStruct(User{})
	if !errs.HasErrors() {
		t.Fatal("expected errors")
	}
	m := errs.Map()
	if _, ok := m["email"]; !ok {
		t.Fatalf("expected field key 'email', got %v", m)
	}
	if _, ok := m["Email"]; ok {
		t.Fatal("should not use Go field name 'Email'")
	}
}

func TestValidateStruct_EqNeOneof(t *testing.T) {
	type Flags struct {
		A string `validate:"eq=yes"`
		B string `validate:"ne=no"`
		C string `validate:"oneof=red green"`
	}

	errs := ValidateStruct(Flags{A: "yes", B: "ok", C: "red"})
	if errs.HasErrors() {
		t.Fatalf("unexpected errors: %v", errs)
	}

	errs = ValidateStruct(Flags{A: "no", B: "no", C: "blue"})
	if len(errs) < 3 {
		t.Fatalf("expected >=3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateStruct_Omitempty(t *testing.T) {
	type User struct {
		Email string `json:"email" validate:"omitempty,email"`
	}

	if errs := ValidateStruct(User{}); errs.HasErrors() {
		t.Fatalf("empty with omitempty should skip email, got %v", errs)
	}
	if errs := ValidateStruct(User{Email: "bad"}); !errs.HasErrors() {
		t.Fatal("invalid email should fail even with omitempty")
	}
}

func TestValidateStruct_Nested(t *testing.T) {
	type Address struct {
		Street string `json:"street" validate:"required,min=2"`
	}
	type User struct {
		Name    string  `json:"name" validate:"required"`
		Address Address `json:"address"`
	}

	errs := ValidateStruct(User{})
	m := errs.Map()
	if _, ok := m["name"]; !ok {
		t.Fatalf("missing name: %v", m)
	}
	if _, ok := m["address.street"]; !ok {
		t.Fatalf("missing address.street: %v", m)
	}

	errs = ValidateStruct(User{Name: "Ada", Address: Address{Street: "AB"}})
	if errs.HasErrors() {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateStruct_NestedPointerOmitempty(t *testing.T) {
	type Address struct {
		Street string `json:"street" validate:"required"`
	}
	type User struct {
		Address *Address `json:"address" validate:"omitempty"`
	}

	if errs := ValidateStruct(User{}); errs.HasErrors() {
		t.Fatalf("nil address omitempty should pass, got %v", errs)
	}
	if errs := ValidateStruct(User{Address: &Address{}}); !errs.HasErrors() {
		t.Fatal("empty street should fail")
	}
}

func TestValidateStruct_Dive(t *testing.T) {
	type Tags struct {
		Items []string `json:"items" validate:"dive,min=2"`
	}

	errs := ValidateStruct(Tags{Items: []string{"ab", "x"}})
	m := errs.Map()
	if _, ok := m["items[1]"]; !ok {
		t.Fatalf("expected items[1] error, got %v", m)
	}

	type Addr struct {
		City string `json:"city" validate:"required"`
	}
	type Profile struct {
		Addresses []Addr `json:"addresses" validate:"dive"`
	}
	errs = ValidateStruct(Profile{Addresses: []Addr{{}, {City: "SP"}}})
	m = errs.Map()
	if _, ok := m["addresses[0].city"]; !ok {
		t.Fatalf("expected addresses[0].city, got %v", m)
	}
}

func TestValidateStruct_RequiredBool(t *testing.T) {
	// Documented behavior: required on bool treats false as missing.
	type Flag struct {
		Active bool `json:"active" validate:"required"`
	}
	if errs := ValidateStruct(Flag{Active: false}); !errs.HasErrors() {
		t.Fatal("false with required should fail (zero-value)")
	}
	if errs := ValidateStruct(Flag{Active: true}); errs.HasErrors() {
		t.Fatalf("true should pass: %v", errs)
	}

	type OptFlag struct {
		Active *bool `json:"active" validate:"required"`
	}
	if errs := ValidateStruct(OptFlag{}); !errs.HasErrors() {
		t.Fatal("nil *bool required should fail")
	}
	f := false
	if errs := ValidateStruct(OptFlag{Active: &f}); errs.HasErrors() {
		t.Fatalf("*false should pass required: %v", errs)
	}
}

func TestValidateStruct_UUID(t *testing.T) {
	type Row struct {
		ID string `json:"id" validate:"uuid"`
	}
	valid := nitrouuid.NewString()
	if errs := ValidateStruct(Row{ID: valid}); errs.HasErrors() {
		t.Fatalf("valid uuid: %v", errs)
	}
	if errs := ValidateStruct(Row{ID: "not-a-uuid"}); !errs.HasErrors() {
		t.Fatal("expected error")
	}

	type V7 struct {
		ID string `validate:"uuid7"`
	}
	if errs := ValidateStruct(V7{ID: valid}); errs.HasErrors() {
		t.Fatalf("nitro uuid is v7: %v", errs)
	}
}

func TestValidateStruct_URLAlphaNumeric(t *testing.T) {
	type Row struct {
		Link string `validate:"url"`
		Name string `validate:"alpha"`
		Code string `validate:"alphanum"`
		Num  string `validate:"numeric"`
	}
	errs := ValidateStruct(Row{
		Link: "https://example.com/x",
		Name: "Ada",
		Code: "Ada42",
		Num:  "3.14",
	})
	if errs.HasErrors() {
		t.Fatalf("unexpected: %v", errs)
	}
	errs = ValidateStruct(Row{
		Link: "ftp://x",
		Name: "Ada1",
		Code: "Ada-1",
		Num:  "x",
	})
	if len(errs) < 4 {
		t.Fatalf("expected >=4 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateStruct_StartsEndsContains(t *testing.T) {
	type Row struct {
		Phone string `validate:"startswith=+55"`
		File  string `validate:"endswith=.go"`
		Msg   string `validate:"contains=ok"`
		Clean string `validate:"excludes= "`
	}
	if errs := ValidateStruct(Row{Phone: "+5511999", File: "a.go", Msg: "all ok", Clean: "x"}); errs.HasErrors() {
		t.Fatalf("unexpected: %v", errs)
	}
	if errs := ValidateStruct(Row{Phone: "119", File: "a.txt", Msg: "nope", Clean: "a b"}); len(errs) < 4 {
		t.Fatalf("expected errors, got %v", errs)
	}
}

func TestValidateStruct_Datetime(t *testing.T) {
	type Row struct {
		Day string `validate:"datetime=2006-01-02"`
	}
	if errs := ValidateStruct(Row{Day: "2026-07-17"}); errs.HasErrors() {
		t.Fatalf("unexpected: %v", errs)
	}
	if errs := ValidateStruct(Row{Day: "17/07/2026"}); !errs.HasErrors() {
		t.Fatal("expected error")
	}
}

func TestValidateStruct_RequiredIf(t *testing.T) {
	type Order struct {
		Status  string `json:"status"`
		Payment string `json:"payment" validate:"required_if=Status paid"`
	}
	if errs := ValidateStruct(Order{Status: "paid", Payment: ""}); len(errs) == 0 {
		t.Fatal("expected required_if error")
	}
	if errs := ValidateStruct(Order{Status: "draft", Payment: ""}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
	if errs := ValidateStruct(Order{Status: "paid", Payment: "card"}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateStruct_RequiredWith(t *testing.T) {
	type Form struct {
		City    string `json:"city"`
		Country string `json:"country" validate:"required_with=City"`
	}
	if errs := ValidateStruct(Form{City: "SP", Country: ""}); len(errs) == 0 {
		t.Fatal("expected required_with error")
	}
	if errs := ValidateStruct(Form{City: "", Country: ""}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateStruct_EqField(t *testing.T) {
	type Acc struct {
		Password string `json:"password"`
		Confirm  string `json:"confirm" validate:"eqfield=Password"`
	}
	if errs := ValidateStruct(Acc{Password: "a", Confirm: "b"}); len(errs) == 0 {
		t.Fatal("expected eqfield error")
	}
	if errs := ValidateStruct(Acc{Password: "a", Confirm: "a"}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateStruct_SameAlias(t *testing.T) {
	type AccSame struct {
		Password string `json:"password"`
		Confirm  string `json:"confirm" validate:"same=Password"`
	}
	type AccEq struct {
		Password string `json:"password"`
		Confirm  string `json:"confirm" validate:"eqfield=Password"`
	}
	type AccSnake struct {
		Password string `json:"password"`
		Confirm  string `json:"confirm" validate:"eq_field=Password"`
	}

	mismatch := struct {
		pw, conf string
	}{"secret", "other"}
	match := struct {
		pw, conf string
	}{"secret", "secret"}

	for _, name := range []string{"same", "eqfield", "eq_field"} {
		t.Run(name+"_mismatch", func(t *testing.T) {
			var errs domain.ValidationErrors
			switch name {
			case "same":
				errs = ValidateStruct(AccSame{Password: mismatch.pw, Confirm: mismatch.conf})
			case "eqfield":
				errs = ValidateStruct(AccEq{Password: mismatch.pw, Confirm: mismatch.conf})
			case "eq_field":
				errs = ValidateStruct(AccSnake{Password: mismatch.pw, Confirm: mismatch.conf})
			}
			if len(errs) == 0 {
				t.Fatal("expected mismatch error")
			}
			if name == "same" && errs[0].Rule != "same" {
				t.Fatalf("want Rule=same, got %q", errs[0].Rule)
			}
		})
		t.Run(name+"_match", func(t *testing.T) {
			var errs domain.ValidationErrors
			switch name {
			case "same":
				errs = ValidateStruct(AccSame{Password: match.pw, Confirm: match.conf})
			case "eqfield":
				errs = ValidateStruct(AccEq{Password: match.pw, Confirm: match.conf})
			case "eq_field":
				errs = ValidateStruct(AccSnake{Password: match.pw, Confirm: match.conf})
			}
			if len(errs) != 0 {
				t.Fatalf("unexpected: %v", errs)
			}
		})
	}
}

func TestValidateStruct_GteField(t *testing.T) {
	type Range struct {
		Start int `json:"start"`
		End   int `json:"end" validate:"gtefield=Start"`
	}
	if errs := ValidateStruct(Range{Start: 10, End: 5}); len(errs) == 0 {
		t.Fatal("expected gtefield error")
	}
	if errs := ValidateStruct(Range{Start: 5, End: 10}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateStruct_Unique(t *testing.T) {
	type Tags struct {
		Items []string `json:"items" validate:"unique"`
	}
	if errs := ValidateStruct(Tags{Items: []string{"a", "a"}}); len(errs) == 0 {
		t.Fatal("expected unique error")
	}
	if errs := ValidateStruct(Tags{Items: []string{"a", "b"}}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

func TestValidateStruct_IP(t *testing.T) {
	type Net struct {
		Addr string `json:"addr" validate:"ipv4"`
	}
	if errs := ValidateStruct(Net{Addr: "999.0.0.1"}); len(errs) == 0 {
		t.Fatal("expected ipv4 error")
	}
	if errs := ValidateStruct(Net{Addr: "127.0.0.1"}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
}

type ageGate struct {
	Age int `json:"age" validate:"gte=0"`
}

func (u ageGate) Validate() error {
	if u.Age > 120 {
		return fmt.Errorf("age too high")
	}
	return nil
}

func TestValidateStruct_CustomValidate(t *testing.T) {
	if errs := ValidateStruct(ageGate{Age: 30}); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
	errs := ValidateStruct(ageGate{Age: 200})
	if len(errs) == 0 {
		t.Fatal("expected custom error")
	}
	if errs[0].Rule != "custom" {
		t.Fatalf("rule = %s", errs[0].Rule)
	}
}
