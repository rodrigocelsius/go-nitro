package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Mock implementations

type mockUserLookup struct {
	users map[string]*domain.User
}

func (m *mockUserLookup) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockUserLookup) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

type mockTokenProvider struct {
	accessTokens  map[string]*domain.Claims // token -> claims
	refreshTokens map[string]string         // refresh token -> user ID
	counter       int
}

func newMockTokenProvider() *mockTokenProvider {
	return &mockTokenProvider{
		accessTokens:  make(map[string]*domain.Claims),
		refreshTokens: make(map[string]string),
		counter:       0,
	}
}

func (m *mockTokenProvider) GenerateAccessToken(ctx context.Context, user *domain.User) (string, error) {
	m.counter++
	token := "access-" + user.ID + "-" + string(rune('A'+m.counter))
	m.accessTokens[token] = &domain.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Roles:  user.Roles,
	}
	return token, nil
}

func (m *mockTokenProvider) GenerateRefreshToken(ctx context.Context, user *domain.User) (string, error) {
	m.counter++
	token := "refresh-" + user.ID + "-" + string(rune('A'+m.counter))
	m.refreshTokens[token] = user.ID
	return token, nil
}

func (m *mockTokenProvider) ValidateToken(ctx context.Context, token string) (*domain.Claims, error) {
	if claims, ok := m.accessTokens[token]; ok {
		return claims, nil
	}
	// Check if it's a refresh token
	if userID, ok := m.refreshTokens[token]; ok {
		return &domain.Claims{UserID: userID}, nil
	}
	return nil, domain.ErrInvalidToken
}

func (m *mockTokenProvider) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	userID, ok := m.refreshTokens[refreshToken]
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	user := &domain.User{ID: userID}
	access, err := m.GenerateAccessToken(ctx, user)
	if err != nil {
		return nil, err
	}
	refresh, err := m.GenerateRefreshToken(ctx, user)
	if err != nil {
		return nil, err
	}
	return &domain.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}, nil
}

func (m *mockTokenProvider) RevokeToken(ctx context.Context, token string) error {
	delete(m.refreshTokens, token)
	return nil
}

type mockPasswordHasher struct {
	verifyCalls int
}

func (m *mockPasswordHasher) Hash(password string) (string, error) {
	return "hashed-" + password, nil
}

func (m *mockPasswordHasher) Verify(password, hash string) bool {
	m.verifyCalls++
	return hash == "hashed-"+password
}

type mockSessionStore struct {
	sessions map[string]*domain.Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]*domain.Session)}
}

func (m *mockSessionStore) Create(ctx context.Context, sess *domain.Session) error {
	m.sessions[sess.ID] = sess
	return nil
}

func (m *mockSessionStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	sess, ok := m.sessions[id]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil, domain.ErrNotFound
	}
	return sess, nil
}

func (m *mockSessionStore) Update(ctx context.Context, sess *domain.Session) error {
	m.sessions[sess.ID] = sess
	return nil
}

func (m *mockSessionStore) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionStore) DeleteByUserID(ctx context.Context, userID string) error {
	for id, sess := range m.sessions {
		if sess.UserID == userID {
			delete(m.sessions, id)
		}
	}
	return nil
}

// Tests

func TestLogin_Success(t *testing.T) {
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password123",
		Name:         "Test User",
		Roles:        []string{"user"},
		Active:       true,
	}

	lookup := &mockUserLookup{users: map[string]*domain.User{user.ID: user}}
	tokenProvider := newMockTokenProvider()
	hasher := &mockPasswordHasher{}
	sessionStore := newMockSessionStore()

	uc := NewAuthUseCase(AuthDeps{
		UserLookup:    lookup,
		TokenProvider: tokenProvider,
		PasswordHash:  hasher,
		SessionStore:  sessionStore,
	})

	pair, err := uc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if pair.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if pair.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", pair.TokenType)
	}
}

func TestLoginOAuth_RejectsUnverifiedEmailForExistingAccount(t *testing.T) {
	existing := &domain.User{ID: "user-1", Email: "victim@example.com"}
	lookup := &mockUserLookup{users: map[string]*domain.User{existing.ID: existing}}
	tokenProvider := newMockTokenProvider()

	uc := NewAuthUseCase(AuthDeps{
		UserLookup:    lookup,
		TokenProvider: tokenProvider,
	})

	// Attacker-controlled OAuth identity claiming the victim's email,
	// but the provider does not confirm the email is verified.
	attacker := &domain.User{ID: "attacker-oauth-id", Email: "victim@example.com", EmailVerified: false}
	pair, err := uc.LoginOAuth(context.Background(), attacker)
	if err != domain.ErrEmailNotVerified {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if pair != nil {
		t.Fatal("expected no token pair to be issued")
	}
}

func TestLoginOAuth_AllowsVerifiedEmailForExistingAccount(t *testing.T) {
	existing := &domain.User{ID: "user-1", Email: "user@example.com", Active: true}
	lookup := &mockUserLookup{users: map[string]*domain.User{existing.ID: existing}}
	tokenProvider := newMockTokenProvider()

	uc := NewAuthUseCase(AuthDeps{
		UserLookup:    lookup,
		TokenProvider: tokenProvider,
	})

	verified := &domain.User{ID: "user-1", Email: "user@example.com", EmailVerified: true}
	pair, err := uc.LoginOAuth(context.Background(), verified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pair == nil || pair.AccessToken == "" {
		t.Fatal("expected a valid token pair")
	}
}

func TestLoginOAuth_AllowsNewUserRegardlessOfVerification(t *testing.T) {
	lookup := &mockUserLookup{users: map[string]*domain.User{}}
	tokenProvider := newMockTokenProvider()

	uc := NewAuthUseCase(AuthDeps{
		UserLookup:    lookup,
		TokenProvider: tokenProvider,
	})

	newOAuthUser := &domain.User{ID: "new-oauth-id", Email: "new@example.com", EmailVerified: false}
	pair, err := uc.LoginOAuth(context.Background(), newOAuthUser)
	if err != nil {
		t.Fatalf("unexpected error creating first-login user: %v", err)
	}
	if pair == nil || pair.AccessToken == "" {
		t.Fatal("expected a valid token pair for first-login registration")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password123",
	}

	lookup := &mockUserLookup{users: map[string]*domain.User{user.ID: user}}
	tokenProvider := newMockTokenProvider()
	hasher := &mockPasswordHasher{}
	sessionStore := newMockSessionStore()

	uc := NewAuthUseCase(AuthDeps{
		UserLookup:    lookup,
		TokenProvider: tokenProvider,
		PasswordHash:  hasher,
		SessionStore:  sessionStore,
	})

	// Wrong password
	_, err := uc.Login(context.Background(), "test@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}

	// Non-existent user
	callsBefore := hasher.verifyCalls
	_, err = uc.Login(context.Background(), "nonexistent@example.com", "password123")
	if hasher.verifyCalls != callsBefore+1 {
		t.Fatalf("Login for a nonexistent user must still call PasswordHash.Verify once "+
			"(V-28, timing-attack mitigation against email enumeration); calls before=%d after=%d",
			callsBefore, hasher.verifyCalls)
	}
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	user := &domain.User{
		ID:     "user-1",
		Email:  "test@example.com",
		Active: true,
	}

	lookup := &mockUserLookup{users: map[string]*domain.User{user.ID: user}}
	tokenProvider := newMockTokenProvider()
	hasher := &mockPasswordHasher{}
	sessionStore := newMockSessionStore()

	uc := NewAuthUseCase(AuthDeps{
		UserLookup:    lookup,
		TokenProvider: tokenProvider,
		PasswordHash:  hasher,
		SessionStore:  sessionStore,
	})

	// Create a refresh token
	refreshToken, _ := tokenProvider.GenerateRefreshToken(context.Background(), user)

	pair, err := uc.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("RefreshToken error: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected new access token")
	}
	// Old refresh token should be revoked
	if _, ok := tokenProvider.refreshTokens[refreshToken]; ok {
		t.Fatal("expected old refresh token to be revoked")
	}
}

func TestLogout_Success(t *testing.T) {
	sessionStore := newMockSessionStore()
	sess := &domain.Session{
		ID:        "session-1",
		UserID:    "user-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = sessionStore.Create(context.Background(), sess)

	uc := NewAuthUseCase(AuthDeps{
		SessionStore: sessionStore,
	})

	err := uc.Logout(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Logout error: %v", err)
	}

	_, err = sessionStore.Get(context.Background(), "session-1")
	if err == nil {
		t.Fatal("expected session to be deleted")
	}
}

func newAuthUC(user *domain.User) (*AuthUseCase, *mockTokenProvider) {
	lookup := &mockUserLookup{users: map[string]*domain.User{user.ID: user}}
	tp := newMockTokenProvider()
	return NewAuthUseCase(AuthDeps{
		UserLookup: lookup, TokenProvider: tp,
		PasswordHash: &mockPasswordHasher{}, SessionStore: newMockSessionStore(),
	}), tp
}

// Regression: disabled accounts could still log in and refresh.
func TestLogin_DisabledAccountRejected(t *testing.T) {
	user := &domain.User{ID: "u1", Email: "a@b.c", PasswordHash: "hashed-pw", Active: false}
	uc, _ := newAuthUC(user)
	if _, err := uc.Login(context.Background(), "a@b.c", "pw"); err == nil {
		t.Fatal("disabled user must not log in")
	}
}

func TestLoginOAuth_DisabledExistingAccountRejected(t *testing.T) {
	user := &domain.User{ID: "u1", Email: "a@b.c", Active: false}
	uc, _ := newAuthUC(user)
	if _, err := uc.LoginOAuth(context.Background(), &domain.User{Email: "a@b.c", EmailVerified: true}); err == nil {
		t.Fatal("disabled user must not log in via OAuth")
	}
}

func TestRefreshToken_DisabledUserRejected(t *testing.T) {
	user := &domain.User{ID: "u1", Email: "a@b.c", Active: true, Roles: []string{"user"}}
	uc, tp := newAuthUC(user)
	rt, _ := tp.GenerateRefreshToken(context.Background(), user)
	user.Active = false // disabled after the refresh token was issued
	if _, err := uc.RefreshToken(context.Background(), rt); err == nil {
		t.Fatal("refresh must fail once the account is disabled")
	}
}

func TestRefreshToken_PicksUpCurrentRoles(t *testing.T) {
	user := &domain.User{ID: "u1", Email: "a@b.c", Active: true, Roles: []string{"admin"}}
	uc, tp := newAuthUC(user)
	rt, _ := tp.GenerateRefreshToken(context.Background(), user)
	user.Roles = []string{"user"} // admin revoked after the token was issued
	pair, err := uc.RefreshToken(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tp.ValidateToken(context.Background(), pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.HasRole("admin") {
		t.Fatalf("revoked role survived refresh: %v", claims.Roles)
	}
}
