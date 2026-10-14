package fasthttp

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/valyala/fasthttp"
)

const (
	oauthStateCookie = "_oauth_state"
	oauthPKCECookie  = "_oauth_pkce"
)

// OAuthProvider defines the interface for OAuth2 providers.
type OAuthProvider interface {
	Name() string
	AuthURL(state string) string
	Callback(ctx context.Context, code string) (*domain.User, error)
}

// pkceOAuthProvider is implemented by providers built via oauth2.New (see
// adapters/auth/oauth2) that support PKCE (RFC 7636). Detected via type
// assertion so OAuthProvider stays a minimal interface and custom
// implementations without PKCE keep working through the plain Callback path.
type pkceOAuthProvider interface {
	AuthURLPKCE(state, verifier string) string
	CallbackPKCE(ctx context.Context, code, verifier string) (*domain.User, error)
}

// OAuthCallbackHandler returns a handler that processes the OAuth2 callback.
// It exchanges the authorization code for tokens, fetches user info,
// and calls LoginOAuth to authenticate/create the user.
func OAuthCallbackHandler(provider OAuthProvider, auth *usecases.AuthUseCase) domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx)

		code := fctx.Query("code")
		if code == "" {
			return &domain.BadRequestError{Message: "missing authorization code"}
		}

		state := fctx.Query("state")
		savedState := fctx.Cookie(oauthStateCookie)
		clearOAuthStateCookie(fctx)
		if savedState == "" || state == "" ||
			subtle.ConstantTimeCompare([]byte(state), []byte(savedState)) != 1 {
			return &domain.BadRequestError{Message: "invalid state parameter"}
		}

		var user *domain.User
		var err error
		if pkceProvider, ok := provider.(pkceOAuthProvider); ok {
			verifier := fctx.Cookie(oauthPKCECookie)
			clearOAuthPKCECookie(fctx)
			if verifier == "" {
				return &domain.BadRequestError{Message: "missing pkce verifier"}
			}
			user, err = pkceProvider.CallbackPKCE(c.Context(), code, verifier)
		} else {
			user, err = provider.Callback(c.Context(), code)
		}
		if err != nil {
			return &domain.UnauthorizedError{Message: "oauth callback failed"}
		}

		tokens, err := auth.LoginOAuth(c.Context(), user)
		if err != nil {
			return &domain.InternalServerError{Err: err}
		}

		return fctx.JSON(200, tokens)
	}
}

// OAuthRedirectHandler returns a handler that redirects to the OAuth2 provider.
// State is stored in an HttpOnly cookie so it survives the provider round-trip.
func OAuthRedirectHandler(provider OAuthProvider) domain.Handler {
	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx)

		state, err := generateState()
		if err != nil {
			return &domain.InternalServerError{Err: err}
		}
		setOAuthStateCookie(fctx, state)

		if pkceProvider, ok := provider.(pkceOAuthProvider); ok {
			verifier, err := generatePKCEVerifier()
			if err != nil {
				return &domain.InternalServerError{Err: err}
			}
			setOAuthPKCECookie(fctx, verifier)
			return fctx.Redirect(pkceProvider.AuthURLPKCE(state, verifier), 302)
		}

		authURL := provider.AuthURL(state)
		return fctx.Redirect(authURL, 302)
	}
}

// OAuthRoutes registers OAuth2 routes for a provider.
//
// Usage:
//
//	app.Get("/auth/google", nitro.OAuthRedirectHandler(google))
//	app.Get("/auth/google/callback", nitro.OAuthCallbackHandler(google, auth))
func OAuthRoutes(prefix string, provider OAuthProvider, auth *usecases.AuthUseCase) (redirect domain.Handler, callback domain.Handler) {
	redirect = OAuthRedirectHandler(provider)
	callback = OAuthCallbackHandler(provider, auth)
	return
}

func setOAuthStateCookie(fc *FastCtx, state string) {
	secure := strings.EqualFold(string(fc.FasthttpCtx().URI().Scheme()), "https")
	fc.SetCookieWithSameSite(oauthStateCookie, state, 600, "/", "", secure, true, fasthttp.CookieSameSiteLaxMode)
}

func clearOAuthStateCookie(fc *FastCtx) {
	fc.SetCookieWithSameSite(oauthStateCookie, "", -1, "/", "", false, true, fasthttp.CookieSameSiteLaxMode)
}

func setOAuthPKCECookie(fc *FastCtx, verifier string) {
	secure := strings.EqualFold(string(fc.FasthttpCtx().URI().Scheme()), "https")
	fc.SetCookieWithSameSite(oauthPKCECookie, verifier, 600, "/", "", secure, true, fasthttp.CookieSameSiteLaxMode)
}

func clearOAuthPKCECookie(fc *FastCtx) {
	fc.SetCookieWithSameSite(oauthPKCECookie, "", -1, "/", "", false, true, fasthttp.CookieSameSiteLaxMode)
}

// generateState creates a cryptographically random state parameter.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// generatePKCEVerifier creates a PKCE code_verifier (RFC 7636): 32 random
// bytes, base64url-encoded without padding — same construction as
// golang.org/x/oauth2's GenerateVerifier, reimplemented here so this
// adapter doesn't need to depend on that package just for OAuthProvider's
// minimal interface.
func generatePKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate pkce verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// EncodeStateURL encodes state for URL parameters.
func EncodeStateURL(state string) string {
	return url.QueryEscape(state)
}
