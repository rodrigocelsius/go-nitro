// cache — Resource reads cached with zero configuration, shared across
// replicas when Redis is available:
//
//	(no REDIS_ADDR)   in-process cache per replica (5 min, singleflight, purged on writes)
//	REDIS_ADDR=…      read-through Redis shared by every replica; writes bust it;
//	                  if Redis goes down, reads go to the database (never a 5xx)
//
// Plus nitro.Remember for any expensive computation.
//
//	cd examples/v2/cache && go test . && go run .
package main

import (
	"context"
	"embed"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Driver struct {
	nitro.Model
	Name    string `json:"name" validate:"required"`
	License string `json:"license" validate:"required"`
}

// cacheFromEnv returns the shared Redis cache when REDIS_ADDR is set, else nil
// (Resources then use their in-process cache).
func cacheFromEnv() nitro.Cache {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil
	}
	return nitro.RedisCache(redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    os.Getenv("REDIS_PASSWORD"),
		DialTimeout: time.Second, // a down Redis must not slow requests down
		ReadTimeout: 500 * time.Millisecond,
	}))
}

func newApp(db *sqladapter.DB, c nitro.Cache) *nitro.App {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	if c != nil {
		app.SetCache(c)
	}
	drivers := sqladapter.StoreFor[Driver](db)
	app.Resource(&Driver{}, drivers, &nitro.ResourceConfig{
		Public: true,
		Queries: nitro.Queries(nitro.Q("count", func(ctx context.Context, _ nitro.Page) (map[string]int, error) {
			// nitro.Remember: cache any computation (nil cache → always computes).
			n, err := nitro.Remember(c, "drivers:count", time.Minute, func() (int, error) {
				return drivers.CountAll(ctx)
			})
			return map[string]int{"drivers": n}, err
		})),
	})
	return app
}

func main() {
	ctx := context.Background()
	var db *sqladapter.DB
	var err error
	if sqladapter.EnvDSN() == "" {
		db, err = sqladapter.Open(ctx, "sqlite", "file:data/cache.db")
	} else {
		db, err = sqladapter.OpenEnv(ctx)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.MigrateFS(ctx, migrations); err != nil {
		log.Fatal(err)
	}
	nitro.Ok(newApp(db, cacheFromEnv()).Listen(":8080"))
}
