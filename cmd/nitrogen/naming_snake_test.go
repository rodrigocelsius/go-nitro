package main

import "testing"

func TestToSnakeAcronyms(t *testing.T) {
	cases := map[string]string{
		"UserID":     "user_id",
		"HTTPServer": "http_server",
		"Product":    "product",
		"ID":         "id",
		"userID":     "user_id",
	}
	for in, want := range cases {
		if got := toSnake(in); got != want {
			t.Errorf("toSnake(%q)=%q want %q", in, got, want)
		}
	}
}
