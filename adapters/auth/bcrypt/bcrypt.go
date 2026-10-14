package bcrypt

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type Hasher struct {
	cost int
}

// MinCost is the lowest bcrypt cost New accepts. bcrypt itself allows a
// cost as low as 4, but that trades away most of the algorithm's intended
// resistance to offline brute-force/GPU cracking if the password
// database ever leaks — 10 (OWASP's floor, and bcrypt.DefaultCost) is
// the minimum this package will silently downgrade to on an invalid
// value. See vuln.md (V-27).
const MinCost = bcrypt.DefaultCost

// New creates a new bcrypt password hasher.
// If no cost is provided, uses bcrypt.DefaultCost. A cost below MinCost
// or above bcrypt.MaxCost is clamped to a safe value instead of being
// passed through, so a misconfigured low cost (e.g. accidentally set to
// 4 for "speed") can't silently weaken every stored password hash.
func New(cost ...int) domain.PasswordHasher {
	c := bcrypt.DefaultCost
	if len(cost) > 0 {
		c = cost[0]
	}
	if c < MinCost || c > bcrypt.MaxCost {
		c = bcrypt.DefaultCost
	}
	return &Hasher{cost: c}
}

func (h *Hasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	return string(bytes), err
}

func (h *Hasher) Verify(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
