package routetestdata

import "github.com/rodrigocelsius/go-nitro/v2"

//nitro:route GET /users/:id
func MismatchedParamName(c *nitro.Ctx, userId int) error {
	return nil
}
