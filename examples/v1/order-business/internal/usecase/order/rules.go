package order

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// NotCancelled runs before every item Command (registered via nitro.Rules).
func NotCancelled(o *model.Order) error {
	if o.Status == "cancelled" {
		return nitro.Conflict("order is cancelled")
	}
	return nil
}

// HighValue gates a Command behind extra roles (used with .When(...).Roles(...)).
func HighValue(o *model.Order) bool {
	return o.Total > 10000
}
