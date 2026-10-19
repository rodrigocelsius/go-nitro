package transform

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Café & Cia. São Paulo!": "cafe-cia-sao-paulo",
		"  Ação  Imediata  ":     "acao-imediata",
		"Straße_Über--Groß":      "strasse-uber-gross",
		"Hello World 2026":       "hello-world-2026",
		"---":                    "",
		"日本":                     "",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFoldAccentsKeepsCase(t *testing.T) {
	if got := FoldAccents("Ação São"); got != "Acao Sao" {
		t.Fatalf("FoldAccents = %q", got)
	}
}
