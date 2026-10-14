package fasthttp

import (
	"context"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/valyala/fasthttp"
)

// fakeUserLookup/fakeTokenProvider are the minimal ports.UserLookup and
// ports.TokenProvider stand-ins needed so LoginOAuth (invoked by
// OAuthCallbackHandler after a successful provider callback) can complete
// without a real persistence/JWT backend.
type fakeUserLookup struct{}

func (fakeUserLookup) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (fakeUserLookup) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}

type fakeTokenProvider struct{}

func (fakeTokenProvider) GenerateAccessToken(ctx context.Context, u *domain.User) (string, error) {
	return "access", nil
}
func (fakeTokenProvider) GenerateRefreshToken(ctx context.Context, u *domain.User) (string, error) {
	return "refresh", nil
}
func (fakeTokenProvider) ValidateToken(ctx context.Context, token string) (*domain.Claims, error) {
	return nil, domain.ErrUnauthorized
}
func (fakeTokenProvider) RefreshToken(ctx context.Context, token string) (*domain.TokenPair, error) {
	return nil, domain.ErrUnauthorized
}
func (fakeTokenProvider) RevokeToken(ctx context.Context, token string) error { return nil }

func newTestAuthUseCase() *usecases.AuthUseCase {
	return usecases.NewAuthUseCase(usecases.AuthDeps{
		UserLookup:    fakeUserLookup{},
		TokenProvider: fakeTokenProvider{},
	})
}

// fakePKCEProvider implements OAuthProvider + pkceOAuthProvider so
// OAuthRedirectHandler/OAuthCallbackHandler exercise the PKCE branch (V-13).
type fakePKCEProvider struct {
	gotVerifier string
}

func (p *fakePKCEProvider) Name() string { return "fake" }
func (p *fakePKCEProvider) AuthURL(state string) string {
	return "https://provider.example/authorize?state=" + state
}
func (p *fakePKCEProvider) Callback(ctx context.Context, code string) (*domain.User, error) {
	return &domain.User{ID: "no-pkce"}, nil
}
func (p *fakePKCEProvider) AuthURLPKCE(state, verifier string) string {
	return "https://provider.example/authorize?state=" + state + "&code_challenge=" + verifier
}
func (p *fakePKCEProvider) CallbackPKCE(ctx context.Context, code, verifier string) (*domain.User, error) {
	p.gotVerifier = verifier
	return &domain.User{ID: "pkce-user"}, nil
}

// TestOAuthRedirectHandler_UsesPKCEWhenSupported verifies the redirect
// handler detects a PKCE-capable provider, sets a verifier cookie, and
// builds the authorization URL via AuthURLPKCE instead of the plain
// AuthURL (RFC 7636 protection against authorization code interception).
func TestOAuthRedirectHandler_UsesPKCEWhenSupported(t *testing.T) {
	provider := &fakePKCEProvider{}
	app := New()
	app.Get("/auth/fake", OAuthRedirectHandler(provider))

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/auth/fake")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusFound {
		t.Fatalf("expected 302, got %d", ctx.Response.StatusCode())
	}
	loc := string(ctx.Response.Header.Peek("Location"))
	if !strings.Contains(loc, "code_challenge=") {
		t.Fatalf("expected PKCE code_challenge in redirect URL, got %q", loc)
	}
	if !strings.Contains(string(ctx.Response.Header.Peek("Set-Cookie")), oauthPKCECookie) {
		t.Fatalf("expected pkce verifier cookie to be set")
	}
}

// TestOAuthCallbackHandler_PassesVerifierFromCookie verifies the callback
// handler reads the verifier cookie set by the redirect step and forwards
// it to CallbackPKCE — without this, the token exchange would either fail
// (verifier missing) or, if a naive implementation skipped verification,
// defeat the point of PKCE entirely.
func TestOAuthCallbackHandler_PassesVerifierFromCookie(t *testing.T) {
	provider := &fakePKCEProvider{}
	app := New()
	app.Get("/auth/fake/callback", OAuthCallbackHandler(provider, newTestAuthUseCase()))

	// Step 1: redirect sets state + verifier cookies.
	redirectCtx := &fasthttp.RequestCtx{}
	redirectCtx.Request.SetRequestURI("/x")
	app2 := New()
	app2.Get("/x", OAuthRedirectHandler(provider))
	app2.Handler()(redirectCtx)

	var stateCookie, pkceCookie string
	redirectCtx.Response.Header.VisitAllCookie(func(key, value []byte) {
		c := fasthttp.AcquireCookie()
		defer fasthttp.ReleaseCookie(c)
		if err := c.ParseBytes(value); err == nil {
			switch string(key) {
			case oauthStateCookie:
				stateCookie = string(c.Value())
			case oauthPKCECookie:
				pkceCookie = string(c.Value())
			}
		}
	})
	if stateCookie == "" || pkceCookie == "" {
		t.Fatalf("expected state and pkce cookies from redirect step, got state=%q pkce=%q", stateCookie, pkceCookie)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/auth/fake/callback?code=abc&state=" + stateCookie)
	ctx.Request.Header.SetCookie(oauthStateCookie, stateCookie)
	ctx.Request.Header.SetCookie(oauthPKCECookie, pkceCookie)

	app.Handler()(ctx)

	if provider.gotVerifier != pkceCookie {
		t.Fatalf("expected CallbackPKCE to receive verifier %q, got %q", pkceCookie, provider.gotVerifier)
	}
}
