package datagen

import "testing"

func columnsByName(m Model) map[string]Column {
	out := make(map[string]Column, len(m.Columns))
	for _, c := range m.Columns {
		out[c.ColumnName] = c
	}
	return out
}

func findModel(t *testing.T, models []Model, name string) Model {
	t.Helper()
	for _, m := range models {
		if m.StructName == name {
			return m
		}
	}
	t.Fatalf("model %s not found", name)
	return Model{}
}

func TestParseModels_ResolvesSiblingFileEmbedAndHardModel(t *testing.T) {
	models, err := ParseModels("testdata/embeds/user.go")
	if err != nil {
		t.Fatal(err)
	}
	cols := columnsByName(findModel(t, models, "Member"))
	if _, ok := cols["scope_id"]; !ok {
		t.Fatalf("promoted scope_id missing: %v", cols)
	}
	if _, ok := cols["deleted_at"]; ok {
		t.Fatalf("HardModel base must not inject deleted_at: %v", cols)
	}
	for _, c := range []string{"id", "created_at", "updated_at", "email", "meta", "tags"} {
		if _, ok := cols[c]; !ok {
			t.Fatalf("column %s missing: %v", c, cols)
		}
	}
}

func TestParseModels_TenantModelInjectsTenantID(t *testing.T) {
	models, err := ParseModels("testdata/embeds/user.go")
	if err != nil {
		t.Fatal(err)
	}
	cols := columnsByName(findModel(t, models, "Order"))
	if _, ok := cols["tenant_id"]; !ok {
		t.Fatalf("TenantModel must add tenant_id: %v", cols)
	}
	if _, ok := cols["deleted_at"]; !ok {
		t.Fatalf("TenantModel keeps soft delete: %v", cols)
	}
}

func TestParseModels_RecordsImports(t *testing.T) {
	models, err := ParseModels("testdata/embeds/user.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := findModel(t, models, "Member").Imports["nitro"]; got != "github.com/rodrigocelsius/go-nitro/pkg/v2" {
		t.Fatalf("nitro import = %q", got)
	}
}

func TestParseModels_SkipsAbstractBase(t *testing.T) {
	models, err := ParseModels("testdata/embeds/base.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if m.StructName == "ScopedModel" {
			t.Fatalf("embedded base must not become a table: %+v", m)
		}
	}
}

func TestParseModels_HonorsTableNameMethod(t *testing.T) {
	models, err := ParseModels("testdata/embeds/user.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := findModel(t, models, "Ledger").TableName; got != "finance_ledger" {
		t.Fatalf("Ledger table = %q, want finance_ledger", got)
	}
	if got := findModel(t, models, "Audit").TableName; got != "comment_wins" {
		t.Fatalf("Audit table = %q, want comment_wins (comment override wins)", got)
	}
}
