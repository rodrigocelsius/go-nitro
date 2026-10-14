package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port        int    `env:"PORT" default:"8080"`
	Env         string `env:"NITRO_ENV" default:"production"`
	ServiceName string `env:"SERVICE_NAME" default:"app"`
}

func Load() (Config, error) {
	return nitro.ConfigOf[Config]()
}
