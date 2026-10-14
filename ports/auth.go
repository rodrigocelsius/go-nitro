package ports

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Authenticator verifies credentials and returns a user.
type Authenticator interface {
	Authenticate(ctx context.Context, email, password string) (*domain.User, error)
}

// TokenProvider generates and validates tokens.
type TokenProvider interface {
	GenerateAccessToken(ctx context.Context, user *domain.User) (string, error)
	GenerateRefreshToken(ctx context.Context, user *domain.User) (string, error)
	ValidateToken(ctx context.Context, token string) (*domain.Claims, error)
	RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error)
	RevokeToken(ctx context.Context, token string) error
}

// UserLookup retrieves users by email or ID.
type UserLookup interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

// UserCreator persists new users.
type UserCreator interface {
	Create(ctx context.Context, user *domain.User) error
}
