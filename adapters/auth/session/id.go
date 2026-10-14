package session

import (
	"crypto/rand"
	"encoding/base64"
)

// idBytes is 256 bits of entropy — comfortably above OWASP's minimum
// (128 bits) for session identifiers.
const idBytes = 32

// NewID returns a cryptographically random, URL-safe session identifier.
// Nitro does not ship a default session ID generator elsewhere in the
// framework, so an application wiring its own domain.Session{ID: ...}
// could otherwise reach for something predictable (a counter, a
// timestamp, math/rand) and make sessions guessable/hijackable
// (CWE-330). MemoryStore.Create and RedisStore.Create call this
// automatically when sess.ID is empty, so callers get a secure ID by
// default too.
func NewID() string {
	var b [idBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read only fails if the OS entropy source is
		// unavailable, which does not happen on supported platforms.
		panic("session: crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
