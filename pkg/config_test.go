package nitro

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMustConfig_DefaultsAndRequired(t *testing.T) {
	t.Setenv("PORT", "")
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("JWT_SECRET")

	type Cfg struct {
		Port      string `env:"PORT" default:"8080"`
		JWTSecret string `env:"JWT_SECRET" validate:"required"`
	}

	if _, err := ConfigOf[Cfg](); err == nil {
		t.Fatal("expected missing JWT_SECRET")
	}

	t.Setenv("JWT_SECRET", "s3cret")
	cfg := MustConfig[Cfg]()
	if cfg.Port != "8080" {
		t.Fatalf("port = %q", cfg.Port)
	}
	if cfg.JWTSecret != "s3cret" {
		t.Fatalf("secret = %q", cfg.JWTSecret)
	}
}

func TestConfigOf_NestedCSVAlias(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	_ = os.Chdir(dir)

	for _, k := range []string{"DB_DSN", "DATABASE_URL", "CORS_ALLOWED_ORIGINS", "APP_NAME", "NEST_HOST"} {
		_ = os.Unsetenv(k)
	}

	type Nest struct {
		Host string `env:"HOST" default:"localhost"`
	}
	type Cfg struct {
		Name    string   `env:"APP_NAME" default:"demo"`
		DSN     string   `env:"DB_DSN,DATABASE_URL" validate:"required"`
		Origins []string `env:"CORS_ALLOWED_ORIGINS" default:"http://localhost:3000"`
		DB      Nest     `env:"NEST_"`
	}

	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://a.test, http://b.test")
	t.Setenv("NEST_HOST", "db.local")

	cfg, err := ConfigOf[Cfg]()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DSN != "postgres://x" {
		t.Fatalf("dsn=%q", cfg.DSN)
	}
	if len(cfg.Origins) != 2 || cfg.Origins[0] != "http://a.test" {
		t.Fatalf("origins=%#v", cfg.Origins)
	}
	if cfg.DB.Host != "db.local" {
		t.Fatalf("host=%q", cfg.DB.Host)
	}
	if cfg.Name != "demo" {
		t.Fatalf("name=%q", cfg.Name)
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DOTENV_TEST_KEY=hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("DOTENV_TEST_KEY")
	LoadDotEnv(path)
	if got := os.Getenv("DOTENV_TEST_KEY"); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestConfigOf_DurationURLOverwrite(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	_ = os.Chdir(dir)

	if err := os.WriteFile(".env", []byte("TIMEOUT=5s\nAPI_URL=http://from-env.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TIMEOUT", "1s")
	t.Setenv("API_URL", "http://from-process.local")

	type Cfg struct {
		Timeout time.Duration `env:"TIMEOUT"`
		API     url.URL       `env:"API_URL,overwrite"`
	}
	cfg, err := ConfigOf[Cfg]()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != time.Second {
		t.Fatalf("timeout=%v (process env should win without overwrite)", cfg.Timeout)
	}
	if cfg.API.Host != "from-env.local" {
		t.Fatalf("api=%v (overwrite should prefer .env)", cfg.API)
	}
}

func TestIDKey(t *testing.T) {
	if IDKey(42) != "42" {
		t.Fatal(IDKey(42))
	}
}

func TestTestClient_PostJSONAndAssert(t *testing.T) {
	app := New()
	app.Post("/echo", func(c *Ctx) error {
		var m map[string]any
		if err := c.BodyJSON(&m); err != nil {
			return err
		}
		return c.OK(m)
	})

	tc := NewTestClient(app)
	resp := tc.PostJSON(t, "/echo", map[string]any{"name": "Ada"})
	tc.AssertStatus(t, resp, 200)
	AssertField(t, resp, "name", "Ada")
	out := AssertJSON[map[string]any](t, resp)
	if out["name"] != "Ada" {
		t.Fatalf("%v", out)
	}
}
