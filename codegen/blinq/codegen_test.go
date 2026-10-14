package blinqgen_test

import (
	"go/format"
	"strings"
	"testing"

	blinqgen "github.com/rodrigocelsius/go-nitro/codegen/blinq/v2"
)

func TestGenerate_TypedFieldsAndEntity(t *testing.T) {
	models, err := blinqgen.ParseModels("../data/testdata/models.go")
	if err != nil {
		t.Fatal(err)
	}
	code, err := blinqgen.Generate("testdata", "github.com/rodrigocelsius/go-nitro/blinq/v2/field", models)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("invalid Go: %v\n%s", err, code)
	}
	s := string(formatted)
	checks := []string{
		"var UserEntity = blinqfield.Entity{",
		`Name:       "users"`,
		`SoftColumn: "deleted_at"`,
		"ID        blinqfield.Field[int]",
		"Name      blinqfield.Field[string]",
		"Age       blinqfield.Field[int]",
		"var UserModel = UserMeta{",
		"var Users = UserModel",
		"order_items_custom",
		`func (User) TableName() string { return "users" }`,
		`func (OrderItem) TableName() string { return "order_items_custom" }`,
	}
	for _, c := range checks {
		if !strings.Contains(s, c) {
			t.Errorf("missing %q in:\n%s", c, s)
		}
	}
}

func TestGenerateDSL_SkipsTableName(t *testing.T) {
	models, err := blinqgen.ParseModels("../data/testdata/models.go")
	if err != nil {
		t.Fatal(err)
	}
	code, err := blinqgen.GenerateDSL("modelgen", "github.com/rodrigocelsius/go-nitro/blinq/v2/field", models)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(code, "TableName()") {
		t.Fatal("DSL output must not include TableName")
	}
	if !strings.Contains(code, "package modelgen") {
		t.Fatal(code)
	}
	if !strings.Contains(code, "var Users = UserModel") {
		t.Fatal(code)
	}
}

func TestGenerateTableNames(t *testing.T) {
	models, err := blinqgen.ParseModels("../data/testdata/models.go")
	if err != nil {
		t.Fatal(err)
	}
	code, err := blinqgen.GenerateTableNames("model", models)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, `func (User) TableName() string { return "users" }`) {
		t.Fatal(code)
	}
	if strings.Contains(code, "UserEntity") {
		t.Fatal("tablename file must not include DSL entity")
	}
}
