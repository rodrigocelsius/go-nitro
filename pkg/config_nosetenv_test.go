package nitro

import (
	"os"
	"testing"
)

func TestConfigOf_DoesNotMutateProcessEnv(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	key := "NITRO_CFG_SIDE_EFFECT_TEST"
	_ = os.Unsetenv(key)
	if err := os.WriteFile(".env", []byte(key+"=from-dotenv\n"), 0644); err != nil {
		t.Fatal(err)
	}
	type Cfg struct {
		Val string `env:"NITRO_CFG_SIDE_EFFECT_TEST"`
	}
	cfg, err := ConfigOf[Cfg]()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Val != "from-dotenv" {
		t.Fatalf("cfg.Val=%q", cfg.Val)
	}
	if _, ok := os.LookupEnv(key); ok {
		t.Fatal("ConfigOf must not os.Setenv")
	}
}
