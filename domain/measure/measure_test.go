package measure_test

import (
	"math"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2/measure"
)

func TestConvertYardsMeters(t *testing.T) {
	m := measure.Meters(10)
	yd, err := measure.Convert(m, measure.Yard)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(yd.Val-10.9361329834) > 0.001 {
		t.Fatalf("got %v yd", yd.Val)
	}
	back, err := measure.Convert(yd, measure.Meter)
	if err != nil || math.Abs(back.Val-10) > 0.001 {
		t.Fatalf("back: %v %v", back, err)
	}
}

func TestIncompatible(t *testing.T) {
	_, err := measure.Convert(measure.Meters(1), measure.Kilogram)
	if err != measure.ErrIncompatibleUnit {
		t.Fatalf("got %v", err)
	}
}
