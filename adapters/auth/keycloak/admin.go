package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// User is a Keycloak user representation.
type User struct {
	ID              string       `json:"id,omitempty"`
	Username        string       `json:"username"`
	Email           string       `json:"email"`
	FirstName       string       `json:"firstName"`
	LastName        string       `json:"lastName"`
	Enabled         bool         `json:"enabled"`
	EmailVerified   bool         `json:"emailVerified"`
	Credentials     []Credential `json:"credentials,omitempty"`
	RequiredActions []string     `json:"requiredActions,omitempty"`
}

// Credential is a Keycloak credential payload.
type Credential struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

// Role is a Keycloak realm role.
type Role struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

// AdminClient interacts with the Keycloak Admin REST API.
type AdminClient interface {
	ListUsers(ctx context.Context, first, max int) ([]User, error)
	CreateUser(ctx context.Context, user User) (string, error)
	UpdateUser(ctx context.Context, userID string, user User) error
	DeleteUser(ctx context.Context, userID string) error
	GetUserByID(ctx context.Context, userID string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	SetUserPassword(ctx context.Context, userID, password string, temporary bool) error
	GetUserRoles(ctx context.Context, userID string) ([]Role, error)
	AssignRole(ctx context.Context, userID, roleName string) error
	RemoveRole(ctx context.Context, userID, roleName string) error
	CreateRole(ctx context.Context, roleName string) error
}

type adminClient struct {
	cfg    Config
	client *http.Client

	mu    sync.Mutex
	token string
}

// NewAdmin creates a Keycloak Admin API client (client_credentials).
func NewAdmin(cfg Config) AdminClient {
	return &adminClient{cfg: cfg, client: httpClient(cfg)}
}

func (a *adminClient) ListUsers(ctx context.Context, first, max int) ([]User, error) {
	if max <= 0 {
		max = 50
	}
	u := fmt.Sprintf("%s?first=%d&max=%d", a.cfg.AdminUsersEndpoint(), first, max)
	var users []User
	if err := a.doJSON(ctx, http.MethodGet, u, nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (a *adminClient) CreateUser(ctx context.Context, user User) (string, error) {
	resp, err := a.doRaw(ctx, http.MethodPost, a.cfg.AdminUsersEndpoint(), user)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("keycloak: user already exists")
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak create user status %d: %s", resp.StatusCode, body)
	}
	location := resp.Header.Get("Location")
	parts := strings.Split(location, "/")
	return parts[len(parts)-1], nil
}

func (a *adminClient) UpdateUser(ctx context.Context, userID string, user User) error {
	u := fmt.Sprintf("%s/%s", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	return a.doJSON(ctx, http.MethodPut, u, user, nil)
}

func (a *adminClient) DeleteUser(ctx context.Context, userID string) error {
	u := fmt.Sprintf("%s/%s", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	return a.doJSON(ctx, http.MethodDelete, u, nil, nil)
}

func (a *adminClient) GetUserByID(ctx context.Context, userID string) (*User, error) {
	u := fmt.Sprintf("%s/%s", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	var user User
	if err := a.doJSON(ctx, http.MethodGet, u, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (a *adminClient) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u := fmt.Sprintf("%s?username=%s&exact=true", a.cfg.AdminUsersEndpoint(), url.QueryEscape(username))
	var users []User
	if err := a.doJSON(ctx, http.MethodGet, u, nil, &users); err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("keycloak: user not found")
	}
	return &users[0], nil
}

func (a *adminClient) SetUserPassword(ctx context.Context, userID, password string, temporary bool) error {
	u := fmt.Sprintf("%s/%s/reset-password", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	payload := Credential{Type: "password", Value: password, Temporary: temporary}
	return a.doJSON(ctx, http.MethodPut, u, payload, nil)
}

func (a *adminClient) GetUserRoles(ctx context.Context, userID string) ([]Role, error) {
	u := fmt.Sprintf("%s/%s/role-mappings/realm", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	var roles []Role
	if err := a.doJSON(ctx, http.MethodGet, u, nil, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func (a *adminClient) AssignRole(ctx context.Context, userID, roleName string) error {
	role, err := a.getRoleByName(ctx, roleName)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			if createErr := a.CreateRole(ctx, roleName); createErr != nil {
				return createErr
			}
			role, err = a.getRoleByName(ctx, roleName)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	u := fmt.Sprintf("%s/%s/role-mappings/realm", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	return a.doJSON(ctx, http.MethodPost, u, []Role{*role}, nil)
}

func (a *adminClient) RemoveRole(ctx context.Context, userID, roleName string) error {
	role, err := a.getRoleByName(ctx, roleName)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil
		}
		return err
	}
	u := fmt.Sprintf("%s/%s/role-mappings/realm", a.cfg.AdminUsersEndpoint(), url.PathEscape(userID))
	return a.doJSON(ctx, http.MethodDelete, u, []Role{*role}, nil)
}

func (a *adminClient) CreateRole(ctx context.Context, roleName string) error {
	return a.doJSON(ctx, http.MethodPost, a.cfg.AdminRolesEndpoint(), map[string]string{"name": roleName}, nil)
}

func (a *adminClient) getRoleByName(ctx context.Context, roleName string) (*Role, error) {
	u := fmt.Sprintf("%s/%s", a.cfg.AdminRolesEndpoint(), url.PathEscape(roleName))
	var role Role
	if err := a.doJSON(ctx, http.MethodGet, u, nil, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (a *adminClient) getToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" {
		return a.token, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", a.cfg.ClientID)
	form.Set("client_secret", a.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.TokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("keycloak: realm %s not found at %s", a.cfg.Realm, a.cfg.BaseURL)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak admin token status %d: %s", resp.StatusCode, body)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	a.token = tr.AccessToken
	return a.token, nil
}

func (a *adminClient) doJSON(ctx context.Context, method, urlStr string, body any, dest any) error {
	resp, err := a.doRaw(ctx, method, urlStr, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak admin %s %s: %d %s", method, urlStr, resp.StatusCode, b)
	}
	if dest != nil {
		return json.NewDecoder(resp.Body).Decode(dest)
	}
	return nil
}

func (a *adminClient) doRaw(ctx context.Context, method, urlStr string, body any) (*http.Response, error) {
	token, err := a.getToken(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		a.mu.Lock()
		a.token = ""
		a.mu.Unlock()
		resp.Body.Close()
		return a.doRaw(ctx, method, urlStr, body)
	}
	return resp, nil
}
