package testdata

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

type Item struct {
	SKU   string  `json:"sku"`
	Price float64 `json:"price"`
}

type Order struct {
	ID       int      `json:"id"`
	Total    float64  `json:"total"`
	Active   bool     `json:"active"`
	Address  Address  `json:"address"`
	Billing  *Address `json:"billing"`
	Items    []Item   `json:"items"`
	Tags     []string `json:"tags"`
	Internal string   // no json tag; generator defaults to snake_case: "internal"
	Secret   string   `json:"-"` // explicit exclusion; generator must ignore it
}
