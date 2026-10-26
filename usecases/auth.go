package usecases

import (
	"context"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

// AuthDeps contains dependencies for auth use cases.
type AuthDeps struct {
	UserLookup  ports.UserLookup
	UserCreator ports.UserCreator
	// Authenticator is deprecated and unused. Login uses UserLookup + PasswordHash.
	// Kept for backward compatibility with older wiring.
	Authenticator ports.Authenticator
	TokenProvider ports.TokenProvider
	PasswordHash  domain.PasswordHasher
	SessionStore  domain.SessionStore
}

// AuthUseCase implements authentication business logic.
type AuthUseCase struct {
	deps AuthDeps
}

// NewAuthUseCase creates a new auth use case.
func NewAuthUseCase(deps AuthDeps) *AuthUseCase {
	return &AuthUseCase{deps: deps}
}

// Claims validates an access token issued by this use case and returns its
// claims (e.g. to attach the user to a device right after Login).
func (a *AuthUseCase) Claims(ctx context.Context, accessToken string) (*domain.Claims, error) {
	if a.deps.TokenProvider == nil {
		return nil, domain.NewUnauthorized("no token provider")
	}
	return a.deps.TokenProvider.ValidateToken(ctx, accessToken)
}

// dummyPasswordHash is a valid bcrypt hash of an arbitrary fixed string,
// used only to burn roughly the same CPU time as a real Verify call when
// no user was found — see the timing-attack note in Login below. Never
// matches any real password.
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Login authenticates a user and returns tokens.
func (a *AuthUseCase) Login(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	email = transform.NormalizeEmail(email)
	// 1. Find user
	user, err := a.deps.UserLookup.FindByEmail(ctx, email)
	if err != nil || user == nil {
		// Still run a bcrypt comparison against a dummy hash so a
		// nonexistent email doesn't return measurably faster than a
		// wrong password for an existing one — without this, an
		// attacker can enumerate valid emails purely from response
		// timing (CWE-208 / CWE-204), since bcrypt.Verify dominates
		// request latency by design.
		a.deps.PasswordHash.Verify(password, dummyPasswordHash)
		return nil, domain.ErrInvalidCredentials
	}

	// 2. Verify password
	if !a.deps.PasswordHash.Verify(password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}

	// 3. Disabled accounts cannot log in. Checked AFTER the password so the
	// response does not reveal which e-mails exist/are disabled to anonymous
	// callers; the generic error avoids leaking the state either way.
	if !user.Active {
		return nil, domain.ErrInvalidCredentials
	}

	// 4. Generate tokens
	return a.generateTokenPair(ctx, user)
}

// Register creates a new user and returns tokens.
func (a *AuthUseCase) Register(ctx context.Context, email, password, name string) (*domain.TokenPair, error) {
	// E-mails are case-insensitive identities: "Ana@x.com" and "ana@x.com"
	// must never become two accounts.
	email = transform.NormalizeEmail(email)
	// 1. Check if email already exists
	existing, _ := a.deps.UserLookup.FindByEmail(ctx, email)
	if existing != nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	// 2. Hash password
	hash, err := a.deps.PasswordHash.Hash(password)
	if err != nil {
		return nil, domain.NewInternal(err)
	}

	// 3. Create user
	user := &domain.User{
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Active:       true,
		Roles:        []string{"user"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 4. Persist user
	if a.deps.UserCreator != nil {
		if err := a.deps.UserCreator.Create(ctx, user); err != nil {
			return nil, err
		}
	}

	// 5. Generate tokens
	return a.generateTokenPair(ctx, user)
}

// LoginOAuth authenticates a user via OAuth2 and returns tokens.
// If the user doesn't exist, it is created automatically (first-login registration).
// The user parameter comes from the OAuth2 provider's userinfo endpoint.
func (a *AuthUseCase) LoginOAuth(ctx context.Context, user *domain.User) (*domain.TokenPair, error) {
	user.Email = transform.NormalizeEmail(user.Email)
	// 1. Try to find existing user by email
	existing, _ := a.deps.UserLookup.FindByEmail(ctx, user.Email)

	if existing != nil {
		// Refuse to link/authenticate into an already-existing account
		// unless the provider confirms the email is verified — otherwise
		// an attacker could claim a victim's email at a provider that
		// allows unverified addresses and take over the local account
		// (account takeover via unverified OAuth email, CWE-287/CWE-290).
		if !user.EmailVerified {
			return nil, domain.ErrEmailNotVerified
		}
		if !existing.Active {
			return nil, domain.ErrInvalidCredentials
		}
		// User exists — merge OAuth ID if empty
		if existing.ID == "" {
			existing.ID = user.ID
		}
		return a.generateTokenPair(ctx, existing)
	}

	// 2. User not found — create new user (first login)
	now := time.Now()
	newUser := &domain.User{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Active:    true,
		Roles:     []string{"user"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 3. Persist new user
	if a.deps.UserCreator != nil {
		if err := a.deps.UserCreator.Create(ctx, newUser); err != nil {
			return nil, err
		}
	}

	// 4. Generate tokens
	return a.generateTokenPair(ctx, newUser)
}

// RefreshToken validates a refresh token and returns new tokens.
func (a *AuthUseCase) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	if a.deps.TokenProvider == nil {
		return nil, domain.ErrUnauthorized
	}
	// TokenProvider.RefreshToken validates type=refresh (ValidateToken only accepts access).
	pair, err := a.deps.TokenProvider.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	_ = a.deps.TokenProvider.RevokeToken(ctx, refreshToken)

	// Re-check the account on every refresh. The token only carries the
	// roles/tenant it was issued with: without this a disabled user (or one
	// whose roles were revoked) keeps minting tokens until the refresh token
	// expires. Fresh tokens are issued from the CURRENT user record.
	if a.deps.UserLookup == nil {
		return pair, nil
	}
	claims, err := a.deps.TokenProvider.ValidateToken(ctx, pair.AccessToken)
	if err != nil || claims == nil || claims.UserID == "" {
		a.revokePair(ctx, pair)
		return nil, domain.ErrUnauthorized
	}
	user, err := a.deps.UserLookup.FindByID(ctx, claims.UserID)
	if err != nil || user == nil || !user.Active {
		a.revokePair(ctx, pair)
		return nil, domain.ErrUnauthorized
	}
	a.revokePair(ctx, pair) // superseded by tokens built from the current user
	return a.generateTokenPair(ctx, user)
}

func (a *AuthUseCase) revokePair(ctx context.Context, pair *domain.TokenPair) {
	_ = a.deps.TokenProvider.RevokeToken(ctx, pair.RefreshToken)
	_ = a.deps.TokenProvider.RevokeToken(ctx, pair.AccessToken)
}

// RevokeTokens invalidates the given access/refresh tokens (logout for
// token-based clients). Invalid or empty tokens are ignored.
func (a *AuthUseCase) RevokeTokens(ctx context.Context, tokens ...string) {
	if a.deps.TokenProvider == nil {
		return
	}
	for _, t := range tokens {
		if t != "" {
			_ = a.deps.TokenProvider.RevokeToken(ctx, t)
		}
	}
}

// Logout deletes a session.
func (a *AuthUseCase) Logout(ctx context.Context, sessionID string) error {
	return a.deps.SessionStore.Delete(ctx, sessionID)
}

// generateTokenPair creates access and refresh tokens for a user.
func (a *AuthUseCase) generateTokenPair(ctx context.Context, user *domain.User) (*domain.TokenPair, error) {
	accessToken, err := a.deps.TokenProvider.GenerateAccessToken(ctx, user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := a.deps.TokenProvider.GenerateRefreshToken(ctx, user)
	if err != nil {
		return nil, err
	}

	expiresIn := 900 // 15 min, the documented default
	if p, ok := a.deps.TokenProvider.(interface{ AccessTokenTTL() time.Duration }); ok && p.AccessTokenTTL() > 0 {
		expiresIn = int(p.AccessTokenTTL().Seconds())
	}
	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}, nil
}
