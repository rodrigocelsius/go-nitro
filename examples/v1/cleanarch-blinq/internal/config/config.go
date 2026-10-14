package config

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Config struct {
	Port        int    `env:"PORT" default:"8080"`
	Env         string `env:"NITRO_ENV" default:"production"`
	ServiceName string `env:"SERVICE_NAME" default:"cleanarch-blinq"`
	DB          DBConfig
}

func Load() (Config, error) {
	// Export .env / .env.local to the process (real env vars win) so code that
	// reads os.Getenv — sqladapter.OpenEnv, OTEL, REDIS_ADDR — sees the same
	// values as Config. No-op when the files are absent (containers).
	nitro.LoadDotEnv()
	return nitro.ConfigOf[Config]()
}

type DBConfig struct {
	DSN    string `env:"DB_DSN,DATABASE_URL"`
	Schema string `env:"DB_SCHEMA" default:"public"`
}
