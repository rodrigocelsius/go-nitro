package oauth2

import (
	"context"

	goauth2 "golang.org/x/oauth2"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// googleUserInfo is the Google userinfo response.
type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Pic           string `json:"picture"`
}

// Google returns a pre-configured OAuth2 provider for Google.
// Default scopes: openid, email, profile.
func Google(cfg domain.OAuthConfig) *Provider {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile"}
	}
	if cfg.Endpoints.AuthURL == "" {
		cfg.Endpoints.AuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if cfg.Endpoints.TokenURL == "" {
		cfg.Endpoints.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if cfg.Endpoints.UserInfoURL == "" {
		cfg.Endpoints.UserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	}

	return New("google", cfg, googleUserFunc(cfg.Endpoints.UserInfoURL))
}

func googleUserFunc(userInfoURL string) UserFunc {
	return func(ctx context.Context, token *goauth2.Token) (*domain.User, error) {
		var info googleUserInfo
		if err := fetchJSON(ctx, userInfoURL, token.AccessToken, &info); err != nil {
			return nil, err
		}

		return &domain.User{
			ID:            info.ID,
			Email:         info.Email,
			Name:          info.Name,
			EmailVerified: info.VerifiedEmail,
		}, nil
	}
}
