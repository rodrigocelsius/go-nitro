package bcrypt

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHasher_Hash(t *testing.T) {
	h := New()
	hash, err := h.Hash("password123")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "password123" {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestHasher_Verify(t *testing.T) {
	h := New()
	hash, err := h.Hash("secret")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if !h.Verify("secret", hash) {
		t.Fatal("expected Verify to return true for correct password")
	}
	if h.Verify("wrong", hash) {
		t.Fatal("expected Verify to return false for wrong password")
	}
}

func TestHasher_CustomCost(t *testing.T) {
	h := New(12) // higher cost
	hash, err := h.Hash("test")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if !h.Verify("test", hash) {
		t.Fatal("expected Verify to return true")
	}
}

// TestNew_ClampsWeakCost covers V-27: a caller passing a cost below the
// recommended floor (e.g. 4, bcrypt's own absolute minimum) must not
// silently weaken every password hash — New clamps back to
// bcrypt.DefaultCost instead of honoring it.
func TestNew_ClampsWeakCost(t *testing.T) {
	h := New(4).(*Hasher)
	if h.cost < MinCost {
		t.Fatalf("cost = %d, want >= %d (MinCost)", h.cost, MinCost)
	}

	tooHigh := New(bcrypt.MaxCost + 5).(*Hasher)
	if tooHigh.cost > bcrypt.MaxCost {
		t.Fatalf("cost = %d, want <= %d (bcrypt.MaxCost)", tooHigh.cost, bcrypt.MaxCost)
	}
}
