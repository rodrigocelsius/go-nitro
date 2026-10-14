package main

import (
	"fmt"

	"github.com/rodrigocelsius/go-nitro/pkg/convert/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type Meta struct {
	SKU string `json:"sku"`
}

// Product demonstrates Nitro portable types.
type Product struct {
	nitro.Model
	Name     string           `json:"name" db:"name"`
	Day      nitro.Date       `json:"availableOn" db:"available_on"`
	Open     nitro.TimeOfDay  `json:"opensAt" db:"opens_at"`
	Meta     nitro.JSON[Meta] `json:"meta" db:"meta"`
	Price    nitro.Money      `json:"price" db:"price"`
	Length   nitro.Measure    `json:"length" db:"length"`
	Location nitro.LatLng     `json:"location" db:"location"`
}

func main() {
	nitro.SetLocale("pt-BR")

	p := Product{
		Name:     "Cabo",
		Day:      nitro.NewDate(2026, 8, 5),
		Open:     nitro.NewTimeOfDay(9, 0, 0, 0),
		Meta:     nitro.JSON[Meta]{V: Meta{SKU: "C-10"}},
		Price:    nitro.MoneyFromMajor(19.9, "BRL"),
		Length:   nitro.Meters(10),
		Location: nitro.MustLatLng(-23.5505199, -46.6333094),
	}
	p.ID = nitro.NewUUID()
	p.CreatedAt = nitro.Now()
	p.UpdatedAt = p.CreatedAt

	yd := nitro.MustConvert(p.Length, nitro.Yard)
	fmt.Println("date:", nitro.FormatDate(p.Day, "pt-BR"))
	fmt.Println("money:", nitro.FormatMoney(p.Price, "pt-BR"))
	fmt.Println("yards:", nitro.FormatMeasure(yd, "en-US"))
	fmt.Println("geo:", nitro.FormatLatLng(p.Location, "pt-BR"))

	m, _ := convert.To[nitro.Measure]("10 m", convert.WithUnit(nitro.Foot))
	fmt.Println("convert:", nitro.FormatMeasure(m, "en-US"))

	pt, _ := convert.To[nitro.LatLng]("-22.9,-43.2")
	fmt.Println("distance_km:", nitro.DistanceMeters(p.Location, pt)/1000)
}
