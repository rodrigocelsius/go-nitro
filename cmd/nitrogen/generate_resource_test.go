package main

import (
	"strings"
	"testing"
)

func TestParseFieldSpec_BlinqIndexSelective(t *testing.T) {
	cases := []struct {
		in      string
		wantIdx bool
	}{
		{"name:string", true},
		{"email:string", true},
		{"sku:string", true},
		{"active:bool", true},
		{"status:string", true},
		{"customerId:string", true},
		{"customer_id:string", true},
		{"price:float64", false},
		{"price:float64:index", true},
		{"description:string", false},
		{"notes:string", false},
		{"password:string", false},
		{"name:string:noindex", false},
		{"qty:int64", false},
		{"created:time", false},
	}
	for _, tc := range cases {
		fs, err := parseFieldSpec(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		has := strings.Contains(fs.Tag, `blinq:"index"`)
		if has != tc.wantIdx {
			t.Fatalf("%s tag=%s want index=%v", tc.in, fs.Tag, tc.wantIdx)
		}
	}
}
