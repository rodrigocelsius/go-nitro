package testdata

// Test model with filterable and sortable fields
type User struct {
	ID        string  `json:"id" db:"id"`
	Name      string  `json:"name" validate:"required"`
	Email     string  `json:"email" unique:"true" validate:"required,email"`
	Role      string  `json:"role" default:"user"`
	Active    bool    `json:"active"`
	Age       int     `json:"age"`
	Price     float64 `json:"price"`
	CreatedAt string  `json:"created_at" db:"created_at"`
	UpdatedAt string  `json:"updated_at" db:"updated_at"`
	DeletedAt string  `json:"deleted_at" db:"deleted_at"`
}
