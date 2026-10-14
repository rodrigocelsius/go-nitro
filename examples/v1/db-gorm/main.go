// DB GORM example — Nitro HTTP + GORM + Gateway (PostgreSQL).
//
// Pool Nitro → Gateway → observed *sql.DB → GORM.
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
package main

import (
	"context"
	"log"
	"strconv"
	"time"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Product struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null" validate:"required,min=2"`
	Price     float64   `json:"price" gorm:"type:numeric(12,2);not null" validate:"gte=0"`
	Active    bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at"`
}

type productStore struct {
	db *gorm.DB
}

func main() {
	ctx := context.Background()
	nitroDB, err := pgxadapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL)", err)
	}
	defer nitroDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: nitroDB.SQL()}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("gorm: %v", err)
	}
	if err := db.AutoMigrate(&Product{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Printf("gorm+gateway: logger=%v pool_ok=%v", nitroDB.Gateway.LoggerEnabled(), nitroDB.Pool.Ping(ctx) == nil)

	store := &productStore{db: db}
	app := nitro.New()
	app.Get("/products", store.list)
	app.Get("/products/:id", store.get)
	app.Post("/products", store.create)
	app.Patch("/products/:id", store.update)
	app.Delete("/products/:id", store.remove)

	nitro.Ok(app.Listen(":8080"))
}

func (s *productStore) list(c *nitro.Ctx) error {
	q := s.db.WithContext(c.RequestContext()).Model(&Product{}).Order("created_at DESC")
	if c.Query("active") == "true" {
		q = q.Where("active = ?", true)
	}
	if min := c.Query("min_price"); min != "" {
		v, err := strconv.ParseFloat(min, 64)
		if err != nil {
			return nitro.BadRequest("min_price must be a number")
		}
		q = q.Where("price >= ?", v)
	}
	if name := c.Query("q"); name != "" {
		q = q.Where("name ILIKE ?", "%"+name+"%")
	}

	var items []Product
	if err := q.Limit(50).Find(&items).Error; err != nil {
		return err
	}
	return c.OK(items)
}

func (s *productStore) get(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	var p Product
	if err := s.db.WithContext(c.RequestContext()).First(&p, id).Error; err != nil {
		return nitro.NotFound("product not found")
	}
	return c.OK(p)
}

func (s *productStore) create(c *nitro.Ctx) error {
	in, err := nitro.Parse[struct {
		Name   string  `json:"name" validate:"required,min=2"`
		Price  float64 `json:"price" validate:"gte=0"`
		Active *bool   `json:"active"`
	}](c)
	if err != nil {
		return err
	}
	p := Product{Name: in.Name, Price: in.Price, Active: true}
	if in.Active != nil {
		p.Active = *in.Active
	}
	if err := s.db.WithContext(c.RequestContext()).Create(&p).Error; err != nil {
		return err
	}
	return c.Created(p)
}

func (s *productStore) update(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	in, err := nitro.Parse[struct {
		Name   string  `json:"name" validate:"required,min=2"`
		Price  float64 `json:"price" validate:"gte=0"`
		Active bool    `json:"active"`
	}](c)
	if err != nil {
		return err
	}
	res := s.db.WithContext(c.RequestContext()).Model(&Product{}).Where("id = ?", id).Updates(Product{
		Name:   in.Name,
		Price:  in.Price,
		Active: in.Active,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nitro.NotFound("product not found")
	}
	return s.get(c)
}

func (s *productStore) remove(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	res := s.db.WithContext(c.RequestContext()).Delete(&Product{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nitro.NotFound("product not found")
	}
	return c.NoContent()
}
