package fasthttp

import (
	"testing"

	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

func TestUserUUID_AndInt64(t *testing.T) {
	c := &FastCtx{}
	c.Set("user_id", nitrouuid.NewString())
	u, err := c.UserUUID()
	if err != nil || u.IsZero() {
		t.Fatalf("UserUUID: %v %v", u, err)
	}

	c2 := &FastCtx{}
	c2.Set("user_id", "42")
	n, err := c2.UserInt64()
	if err != nil || n != 42 {
		t.Fatalf("UserInt64: %v %v", n, err)
	}

	c3 := &FastCtx{}
	if _, err := c3.UserUUID(); err == nil {
		t.Fatal("expected error for missing user")
	}
}
