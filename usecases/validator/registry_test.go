package validator_test

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestRegistry_RegisterLookup(t *testing.T) {
	r := validator.NewRegistry()
	called := false
	r.Register("cpf", func(ctx domain.FieldContext, param string) error {
		called = true
		if ctx.FieldValue != "123" {
			t.Fatalf("FieldValue = %v", ctx.FieldValue)
		}
		return nil
	})

	fn, ok := r.Lookup("CPF") // case-insensitive
	if !ok {
		t.Fatal("Lookup CPF failed")
	}
	ctx := validator.FieldContextFromValues(nil, nil, "CPF", "cpf", "cpf", "123")
	if err := fn(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("validator not called")
	}
}

func TestRegistry_CallUnknown(t *testing.T) {
	r := validator.NewRegistry()
	err := r.Call(domain.FieldContext{}, "missing", "")
	if err == nil || err.Error() != "unknown validation rule: missing" {
		t.Fatalf("got %v", err)
	}
}

func TestRegistry_Alias(t *testing.T) {
	r := validator.NewRegistry()
	r.Register("eqfield", func(ctx domain.FieldContext, param string) error {
		return fmt.Errorf("eqfield:%s:%v", param, ctx.FieldValue)
	})
	r.RegisterAlias("same", "eqfield")

	fn, ok := r.Lookup("same")
	if !ok {
		t.Fatal("alias same not found")
	}
	err := fn(validator.FieldContextFromValues(nil, nil, "Confirm", "confirm", "confirm", "x"), "Password")
	if err == nil || err.Error() != "eqfield:Password:x" {
		t.Fatalf("got %v", err)
	}
}

func TestRegistry_AliasChainAndLoop(t *testing.T) {
	r := validator.NewRegistry()
	r.Register("base", func(ctx domain.FieldContext, param string) error { return errors.New("ok-base") })
	r.RegisterAlias("a", "b")
	r.RegisterAlias("b", "base")
	if _, ok := r.Lookup("a"); !ok {
		t.Fatal("chain a→b→base")
	}
	r.RegisterAlias("loop1", "loop2")
	r.RegisterAlias("loop2", "loop1")
	if _, ok := r.Lookup("loop1"); ok {
		t.Fatal("cycle should not resolve")
	}
}

func TestRegistry_Clone(t *testing.T) {
	r := validator.NewRegistry()
	r.Register("cpf", func(ctx domain.FieldContext, param string) error {
		return errors.New("cpf")
	})
	r.RegisterAlias("doc", "cpf")

	c := r.Clone()
	c.Register("cpf", func(ctx domain.FieldContext, param string) error {
		return errors.New("cloned")
	})

	errOrig := r.Call(domain.FieldContext{}, "doc", "")
	errClone := c.Call(domain.FieldContext{}, "doc", "")
	if errOrig.Error() != "cpf" || errClone.Error() != "cloned" {
		t.Fatalf("orig=%v clone=%v", errOrig, errClone)
	}
}

func TestRegistry_CustomCPFWithoutValidateStruct(t *testing.T) {
	r := validator.NewRegistry()
	r.Register("cpf", func(ctx domain.FieldContext, param string) error {
		s, _ := ctx.FieldValue.(string)
		if s != "529.982.247-25" && s != "52998224725" {
			return errors.New("invalid cpf")
		}
		return nil
	})

	ctx := validator.FieldContextFromValues(
		struct{ CPF string }{CPF: "000"},
		nil, "CPF", "cpf", "cpf", "000",
	)
	if err := r.Call(ctx, "cpf", ""); err == nil {
		t.Fatal("expected invalid cpf")
	}
	ctx.FieldValue = "52998224725"
	if err := r.Call(ctx, "cpf", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := validator.NewRegistry()
	r.Register("base", func(ctx domain.FieldContext, param string) error { return nil })

	var wg sync.WaitGroup
	var fails atomic.Int64
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("rule%d", i)
			r.Register(name, func(ctx domain.FieldContext, param string) error {
				return nil
			})
			r.RegisterAlias(fmt.Sprintf("alias%d", i), name)
			for j := 0; j < 100; j++ {
				if _, ok := r.Lookup(name); !ok {
					fails.Add(1)
				}
				if _, ok := r.Lookup(fmt.Sprintf("alias%d", i)); !ok {
					fails.Add(1)
				}
				_ = r.Call(domain.FieldContext{FieldValue: i}, name, "")
			}
		}(i)
	}
	wg.Wait()
	if fails.Load() != 0 {
		t.Fatalf("lookup failures: %d", fails.Load())
	}
}

func TestDefaultRegistry_RegisterValidator(t *testing.T) {
	prev := validator.DefaultRegistry().Clone()
	defer validator.SetDefaultRegistry(prev)

	validator.SetDefaultRegistry(validator.NewRegistry())
	validator.RegisterValidator("tmp", func(ctx domain.FieldContext, param string) error {
		return errors.New("tmp-ok")
	})
	err := validator.DefaultRegistry().Call(domain.FieldContext{}, "tmp", "")
	if err == nil || err.Error() != "tmp-ok" {
		t.Fatalf("got %v", err)
	}
}

func TestFieldContextHelpers(t *testing.T) {
	type Acc struct {
		Password string
		Confirm  string
	}
	acc := Acc{Password: "a", Confirm: "b"}
	ctx := validator.FieldContextFromValues(acc, acc, "Confirm", "confirm", "confirm", "b")
	if ctx.Parent == nil || ctx.Root == nil || ctx.Path != "confirm" {
		t.Fatalf("%+v", ctx)
	}
}

func TestWrapLegacyRule(t *testing.T) {
	fn := validator.WrapLegacyRule("demo", func(fieldName string, value, parent reflect.Value, arg string) *domain.ValidationError {
		if fieldName != "age" || arg != "18" {
			t.Fatalf("field=%s arg=%s", fieldName, arg)
		}
		if value.Interface() != 10 {
			t.Fatalf("value=%v", value.Interface())
		}
		return &domain.ValidationError{Field: fieldName, Message: "too young", Rule: "demo"}
	})
	err := fn(validator.FieldContextFromValues(nil, nil, "Age", "age", "age", 10), "18")
	if err == nil || err.Error() != "age: too young" {
		t.Fatalf("got %v", err)
	}
}
