package testdata

type User struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int
}

//nitro:table=order_items_custom
type OrderItem struct {
	ID    int
	SKU   string
	Price float64
}

type NoColumns struct {
}
