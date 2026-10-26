package decimal

import (
	"encoding/json"
	"testing"
)

func TestDecimalParseAndFormat(t *testing.T) {
	cases := map[string]string{
		"":        "0",
		" 42 ":    "42",
		"-7":      "-7",
		"1.5":     "1.5000000000",
		"1/4":     "0.2500000000",
		"10.0000": "10",
	}
	for in, want := range cases {
		d, err := NewFromString(in)
		if err != nil || d.String() != want {
			t.Errorf("NewFromString(%q) = %q, %v; want %q", in, d.String(), err, want)
		}
	}
	if _, err := NewFromString("abc"); err == nil {
		t.Fatal("invalid input must fail")
	}
	if !(Decimal{}).IsZero() || NewFromInt64(3).IsZero() {
		t.Fatal("IsZero")
	}
	if f, ok := NewFromFloat64(2.5).Float64(); !ok || f != 2.5 {
		t.Fatalf("Float64 = %v %v", f, ok)
	}
}

func TestDecimalScanValue(t *testing.T) {
	var d Decimal
	for _, v := range []any{"3.25", []byte("3.25"), 3.25, int64(3), nil} {
		if err := d.Scan(v); err != nil {
			t.Fatalf("Scan(%T): %v", v, err)
		}
	}
	if d.String() != "0" {
		t.Fatalf("Scan(nil) = %q", d)
	}
	if err := d.Scan(true); err == nil {
		t.Fatal("unsupported type must fail")
	}
	var nilD *Decimal
	if err := nilD.Scan("1"); err == nil {
		t.Fatal("nil receiver must fail")
	}
	v, _ := NewFromInt64(9).Value()
	if v != "9" {
		t.Fatalf("Value = %v", v)
	}
}

func TestDecimalJSON(t *testing.T) {
	var in struct{ A, B, C Decimal }
	if err := json.Unmarshal([]byte(`{"A":"1.25","B":2,"C":null}`), &in); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(in)
	if string(out) != `{"A":"1.2500000000","B":"2","C":"0"}` {
		t.Fatalf("round trip: %s", out)
	}
	var bad Decimal
	if err := json.Unmarshal([]byte(`"x"`), &bad); err == nil {
		t.Fatal("invalid string must fail")
	}
	if err := json.Unmarshal([]byte(`{}`), &bad); err == nil {
		t.Fatal("object must fail")
	}
}
