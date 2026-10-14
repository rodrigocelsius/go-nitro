package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port        int    `env:"PORT" default:"8080"`
	Env         string `env:"NITRO_ENV" default:"production"`
	ServiceName string `env:"SERVICE_NAME" default:"order-business"`
	// JWTSecret has no default on purpose — a hardcoded fallback would
	// ship the same secret to every deployment of this example (see
	// vuln.md V-21). main.go falls back to a clearly-labeled
	// development-only value only when this is empty.
	JWTSecret string `env:"JWT_SECRET"`
	DB        DBConfig
}

func Load() (Config, error) {
	return nitro.ConfigOf[Config]()
}

type DBConfig struct {
	DSN    string `env:"DB_DSN,DATABASE_URL"`
	Schema string `env:"DB_SCHEMA" default:"public"`
}
