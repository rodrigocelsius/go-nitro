package convert_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2/measure"
	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
	"github.com/rodrigocelsius/go-nitro/pkg/convert/v2"
	"github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

func TestToUUID(t *testing.T) {
	u := uuid.New()
	out, err := convert.To[uuid.UUID](u.String())
	if err != nil || out != u {
		t.Fatalf("%v %v", out, err)
	}
}

func TestToMeasureUnit(t *testing.T) {
	m, err := convert.To[measure.Measure]("10 m", convert.WithUnit(measure.Yard))
	if err != nil {
		t.Fatal(err)
	}
	if m.Unit != measure.Yard {
		t.Fatalf("unit %s", m.Unit)
	}
}

func TestToDate(t *testing.T) {
	d, err := convert.To[temporal.Date]("2026-08-05")
	if err != nil || d.String() != "2026-08-05" {
		t.Fatalf("%v %v", d, err)
	}
}
