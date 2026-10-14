package nitro

import (
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// LoginSession creates a brand-new server-side session for userID and sets
// the session cookie on the response — the secure counterpart to
// SessionMiddleware (see docs/guide/authentication.md#session).
//
// Always call this to establish a session right after a successful login
// (password, OAuth2, …), instead of ever reusing a session ID sent by the
// client before authentication. domain.SessionStore.Create mints the ID via
// crypto/rand whenever Session.ID is empty (which it always is here), so
// the ID issued to an authenticated user can never be one an attacker
// planted in the victim's browser beforehand — this is the standard
// defense against session fixation (CWE-384).
//
//	sess, err := nitro.LoginSession(c, store, user.ID, "session_id", 24*time.Hour)
func LoginSession(c *Ctx, store SessionStore, userID, cookieName string, ttl time.Duration) (*domain.Session, error) {
	sess := &domain.Session{
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
	if err := store.Create(c.Context(), sess); err != nil {
		return nil, err
	}
	c.SetCookie(cookieName, sess.ID, int(ttl.Seconds()), "/", "", true, true)
	return sess, nil
}

// LogoutSession deletes the session referenced by the cookie and clears the
// cookie, so the same session ID cannot be reused after logout.
func LogoutSession(c *Ctx, store SessionStore, cookieName string) error {
	sessionID := c.Cookie(cookieName)
	c.DeleteCookie(cookieName)
	if sessionID == "" {
		return nil
	}
	return store.Delete(c.Context(), sessionID)
}
