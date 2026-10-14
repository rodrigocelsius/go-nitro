package nitro

import "testing"

func TestNew_EnvFromNitroEnv(t *testing.T) {
	t.Setenv("NITRO_ENV", "development")
	if New(Config{DisableTelemetry: true}).cfg.Env != EnvDevelopment {
		t.Fatal("NITRO_ENV=development must select development")
	}
	t.Setenv("NITRO_ENV", "")
	if New(Config{DisableTelemetry: true}).cfg.Env != EnvProduction {
		t.Fatal("unset NITRO_ENV must stay production")
	}
}
