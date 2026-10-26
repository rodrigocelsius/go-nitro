package oauth2

import (
	"context"
	"strconv"

	goauth2 "golang.org/x/oauth2"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// githubUserInfo is the GitHub user response.
type githubUserInfo struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// githubEmail is a GitHub email entry.
type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// GitHub returns a pre-configured OAuth2 provider for GitHub.
// Default scopes: user:email.
func GitHub(cfg domain.OAuthConfig) *Provider {
	// Ensure user:email scope is present
	hasEmail := false
	for _, s := range cfg.Scopes {
		if s == "user:email" {
			hasEmail = true
			break
		}
	}
	if !hasEmail {
		cfg.Scopes = append(cfg.Scopes, "user:email")
	}
	if cfg.Endpoints.AuthURL == "" {
		cfg.Endpoints.AuthURL = "https://github.com/login/oauth/authorize"
	}
	if cfg.Endpoints.TokenURL == "" {
		cfg.Endpoints.TokenURL = "https://github.com/login/oauth/access_token"
	}
	if cfg.Endpoints.UserInfoURL == "" {
		cfg.Endpoints.UserInfoURL = "https://api.github.com/user"
	}

	return New("github", cfg, githubUserFunc(cfg.Endpoints.UserInfoURL))
}

func githubUserFunc(userInfoURL string) UserFunc {
	return func(ctx context.Context, token *goauth2.Token) (*domain.User, error) {
		var info githubUserInfo
		if err := fetchJSON(ctx, userInfoURL, token.AccessToken, &info); err != nil {
			return nil, err
		}

		// GitHub only lets a user publish a verified email as their public
		// profile email, so info.Email (from /user) is verified by
		// construction. If it's empty, fall back to the /user/emails
		// endpoint and track the `verified` flag of whichever entry we pick.
		verified := info.Email != ""
		if info.Email == "" {
			emails, err := fetchGitHubEmails(ctx, userInfoURL+"/emails", token.AccessToken)
			if err == nil {
				for _, e := range emails {
					if e.Primary {
						info.Email = e.Email
						verified = e.Verified
						break
					}
				}
				// Fallback to first email
				if info.Email == "" && len(emails) > 0 {
					info.Email = emails[0].Email
					verified = emails[0].Verified
				}
			}
		}

		name := info.Name
		if name == "" {
			name = info.Login
		}

		return &domain.User{
			ID:            strconv.Itoa(info.ID),
			Email:         info.Email,
			Name:          name,
			EmailVerified: verified,
		}, nil
	}
}

// fetchGitHubEmails reads {UserInfoURL}/emails, so GitHub Enterprise
// (https://ghe.example/api/v3/user) works like github.com.
func fetchGitHubEmails(ctx context.Context, url, accessToken string) ([]githubEmail, error) {
	var emails []githubEmail
	if err := fetchJSON(ctx, url, accessToken, &emails); err != nil {
		return nil, err
	}
	return emails, nil
}
