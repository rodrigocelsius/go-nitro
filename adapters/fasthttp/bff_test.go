package fasthttp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TestResolveUpstreamPath_EscapesParamValues covers V-20: route params
// substituted into the upstream path must be URL-escaped so a
// client-controlled value can't break out of its path segment or inject
// extra query parameters into the upstream request.
func TestResolveUpstreamPath_EscapesParamValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain id", "42", "/orders/42"},
		{"path traversal attempt", "../admin", "/orders/..%2Fadmin"},
		{"query string injection attempt", "1?evil=1", "/orders/1%3Fevil=1"},
		{"space and special chars", "a b/c", "/orders/a%20b%2Fc"},
	}
	for _, tt := range tests {
		c := &domain.Ctx{ParamCount: 1, ParamNames: [8]string{"id"}, ParamValues: [8]string{tt.value}}
		if got := resolveUpstreamPath("/orders/:id", c); got != tt.want {
			t.Errorf("resolveUpstreamPath(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestBFFProxyHandler(t *testing.T) {
	app := New()

	cfg := BFFConfig{
		Name: "test",
		Services: []ServiceConfig{
			{
				Name:    "users",
				BaseURL: "http://localhost:9999",
				Timeout: 5 * time.Second,
			},
		},
		CacheEnabled: false,
	}

	app.Get("/users", BFFProxyHandler(cfg, "users", "/api/users"))

	routes := app.Routes()
	found := false
	for _, r := range routes {
		if r.Path == "/users" && r.Method == "GET" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Route /users not registered")
	}
}

func TestBFFAggregateHandler(t *testing.T) {
	app := New()

	cfg := BFFConfig{
		Name: "test",
		Services: []ServiceConfig{
			{
				Name:    "users",
				BaseURL: "http://localhost:9999",
				Timeout: 5 * time.Second,
			},
			{
				Name:    "orders",
				BaseURL: "http://localhost:9998",
				Timeout: 5 * time.Second,
			},
		},
		CacheEnabled: false,
	}

	requests := []AggregateRequest{
		{Service: "users", Path: "/api/users/me", Key: "user"},
		{Service: "orders", Path: "/api/orders?limit=5", Key: "recent_orders"},
	}

	app.Get("/dashboard", BFFAggregateHandler(cfg, requests))

	routes := app.Routes()
	found := false
	for _, r := range routes {
		if r.Path == "/dashboard" && r.Method == "GET" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Route /dashboard not registered")
	}
}

func TestWebBFFConfig(t *testing.T) {
	cfg := WebBFFConfig{
		BFFConfig: BFFConfig{
			Name: "web",
			Services: []ServiceConfig{
				{Name: "users", BaseURL: "http://localhost:8081"},
			},
		},
		TokenCookieName:   "_at",
		RefreshCookieName: "_rt",
		CSRFCookieName:    "_csrf",
		CookieSecure:      true,
		CookieSameSite:    "Lax",
		CookieHTTPOnly:    true,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   7 * 24 * time.Hour,
		EnableCSRF:        true,
	}

	if cfg.TokenCookieName != "_at" {
		t.Errorf("TokenCookieName = %q, want _at", cfg.TokenCookieName)
	}
	if cfg.RefreshCookieName != "_rt" {
		t.Errorf("RefreshCookieName = %q, want _rt", cfg.RefreshCookieName)
	}
	if !cfg.EnableCSRF {
		t.Error("EnableCSRF should be true by default")
	}
}

func TestMobileBFFConfig(t *testing.T) {
	cfg := MobileBFFConfig{
		BFFConfig: BFFConfig{
			Name: "mobile",
			Services: []ServiceConfig{
				{Name: "users", BaseURL: "http://localhost:8081"},
			},
		},
		TokenHeader:          "Authorization",
		TokenPrefix:          "Bearer ",
		AccessTokenTTL:       30 * time.Minute,
		RefreshTokenTTL:      30 * 24 * time.Hour,
		EnableDeviceTracking: true,
		MaxDevicesPerUser:    5,
		CompactResponse:      true,
	}

	if cfg.TokenHeader != "Authorization" {
		t.Errorf("TokenHeader = %q, want Authorization", cfg.TokenHeader)
	}
	if !cfg.EnableDeviceTracking {
		t.Error("EnableDeviceTracking should be true")
	}
	if cfg.MaxDevicesPerUser != 5 {
		t.Errorf("MaxDevicesPerUser = %d, want 5", cfg.MaxDevicesPerUser)
	}
}

func TestBFFProxyHandlerPanicsOnMissingService(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("BFFProxyHandler should panic when service not found")
		}
	}()

	cfg := BFFConfig{
		Name:     "test",
		Services: []ServiceConfig{},
	}

	BFFProxyHandler(cfg, "nonexistent", "/api/test")
}

func TestTransformRegistry(t *testing.T) {
	registry := NewTransformRegistry()

	registry.Register("GET", "/users", func(c *domain.Ctx, body []byte) []byte {
		return body
	})

	transform := registry.Get("GET", "/users")
	if transform == nil {
		t.Error("Transform not found")
	}

	transform = registry.Get("GET", "/nonexistent")
	if transform != nil {
		t.Error("Transform should be nil for non-existent route")
	}
}

func TestServiceConfig(t *testing.T) {
	svc := ServiceConfig{
		Name:    "users",
		BaseURL: "http://localhost:8081",
		Timeout: 5 * time.Second,
		Headers: map[string]string{
			"X-Custom": "value",
		},
	}

	if svc.Name != "users" {
		t.Errorf("Name = %q, want users", svc.Name)
	}
	if svc.BaseURL != "http://localhost:8081" {
		t.Errorf("BaseURL = %q, want http://localhost:8081", svc.BaseURL)
	}
	if svc.Headers["X-Custom"] != "value" {
		t.Errorf("Headers[X-Custom] = %q, want value", svc.Headers["X-Custom"])
	}
}

func TestBFFRoute(t *testing.T) {
	route := BFFRoute{
		Method:   "GET",
		Path:     "/users",
		Service:  "users",
		Upstream: "/api/users",
		Auth:     true,
		Cache:    5 * time.Minute,
	}

	if route.Method != "GET" {
		t.Errorf("Method = %q, want GET", route.Method)
	}
	if !route.Auth {
		t.Error("Auth should be true")
	}
	if route.Cache != 5*time.Minute {
		t.Errorf("Cache = %v, want 5m", route.Cache)
	}
}

func TestAggregateRequest(t *testing.T) {
	req := AggregateRequest{
		Service: "users",
		Path:    "/api/users/me",
		Key:     "user",
		Method:  "POST",
		Body:    []byte(`{"action":"update"}`),
	}

	if req.Service != "users" {
		t.Errorf("Service = %q, want users", req.Service)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST", req.Method)
	}
	if string(req.Body) != `{"action":"update"}` {
		t.Errorf("Body = %q, want {\"action\":\"update\"}", string(req.Body))
	}
}

func TestDeviceInfo(t *testing.T) {
	device := domain.DeviceInfo{
		DeviceID:   "device-123",
		UserID:     "user-456",
		Platform:   "ios",
		FCMToken:   "fcm-token",
		APNSToken:  "apns-token",
		Model:      "iPhone 14",
		OSVersion:  "16.0",
		AppVersion: "1.2.3",
	}

	if device.DeviceID != "device-123" {
		t.Errorf("DeviceID = %q, want device-123", device.DeviceID)
	}
	if device.Platform != "ios" {
		t.Errorf("Platform = %q, want ios", device.Platform)
	}
}

func TestBFFConfigJSON(t *testing.T) {
	cfg := BFFConfig{
		Name:    "test",
		Version: "1.0.0",
		Services: []ServiceConfig{
			{Name: "users", BaseURL: "http://localhost:8081"},
		},
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal BFFConfig: %v", err)
	}

	var parsed BFFConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal BFFConfig: %v", err)
	}

	if parsed.Name != "test" {
		t.Errorf("Name = %q, want test", parsed.Name)
	}
	if !parsed.CacheEnabled {
		t.Error("CacheEnabled should be true")
	}
}
