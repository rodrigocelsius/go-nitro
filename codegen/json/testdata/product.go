package testdata

import "time"

type Product struct {
	Name       string            `json:"name"`
	Price      float64           `json:"price"`
	Metadata   map[string]string `json:"metadata"`
	Quantities map[string]int    `json:"quantities"`
	CreatedAt  time.Time         `json:"created_at"`
}
