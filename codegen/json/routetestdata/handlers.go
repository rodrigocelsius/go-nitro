package routetestdata

import "github.com/rodrigocelsius/go-nitro/v2"

//nitro:route GET /users/:id
func GetUser(c *nitro.Ctx, id int) error {
	if id < 0 {
		return c.Status(400).SendString("invalid id")
	}
	return c.SendString("user-" + itoa(id))
}

//nitro:route GET /orders/:orderId/items/:itemId
func GetOrderItem(c *nitro.Ctx, orderId int, itemId int) error {
	return c.SendString("order-" + itoa(orderId) + "-item-" + itoa(itemId))
}

//nitro:route GET /products/:slug
func GetProduct(c *nitro.Ctx, slug string) error {
	return c.SendString("product-" + slug)
}

// NotAnnotated has no //nitro:route comment and must be ignored.
func NotAnnotated(c *nitro.Ctx) error {
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
