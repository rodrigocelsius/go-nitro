package testdata

// Test model with various tags
type User struct {
	Name    string `json:"name"  validate:"required"`
	Email   string `json:"email" unique:"true" validate:"required,email"`
	Phone   string `json:"phone" blinq:"unique,where=deleted_at IS NULL"`
	Role    string `json:"role"  default:"user"`
	Active  bool   `json:"active" blinq:"index"`
	Country string `json:"country" blinq:"index"`
}

// Test model with custom ID
type Product struct {
	ID    int     `json:"id" primary:"true"`
	Name  string  `json:"name" validate:"required"`
	Price float64 `json:"price" validate:"required,gt=0"`
}

// Test model with custom table name
//
//nitro:table=custom_orders
type Order struct {
	UserID string  `json:"user_id" validate:"required"`
	Amount float64 `json:"amount" validate:"required,gt=0"`
	Status string  `json:"status" default:"pending"`
}
