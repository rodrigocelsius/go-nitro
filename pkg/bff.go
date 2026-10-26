package nitro

import (
	"time"

	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
)

// === BFF Types ===

// BFFConfig is the base BFF configuration.
type BFFConfig = nitroadapter.BFFConfig

// ServiceConfig configures an upstream service.
type ServiceConfig = nitroadapter.ServiceConfig

// BFFRoute defines a route the BFF proxies to a service.
type BFFRoute = nitroadapter.BFFRoute

// AggregateRequest defines a request for aggregation.
type AggregateRequest = nitroadapter.AggregateRequest

// === BFF Handlers ===

// BFFProxyHandler returns a handler that proxies requests to upstream services.
func BFFProxyHandler(cfg BFFConfig, serviceName, upstreamPath string) Handler {
	mw := nitroadapter.BFFProxyHandler(cfg, serviceName, upstreamPath)
	return wrapAdapter(mw)
}

// BFFAggregateHandler returns a handler that aggregates multiple requests.
func BFFAggregateHandler(cfg BFFConfig, requests []AggregateRequest) Handler {
	mw := nitroadapter.BFFAggregateHandler(cfg, requests)
	return wrapAdapter(mw)
}

// === Web BFF Types ===

// WebBFFConfig configures the BFF for web applications.
type WebBFFConfig = nitroadapter.WebBFFConfig

// === Web BFF Handlers ===

// WebBFFMiddleware returns BFF middleware for web.
func WebBFFMiddleware(cfg WebBFFConfig, tokenProvider ports.TokenProvider) Handler {
	mw := nitroadapter.WebBFFMiddleware(cfg, tokenProvider)
	return wrapAdapter(mw)
}

// WebBFFLoginHandler returns a web BFF login handler.
func WebBFFLoginHandler(auth *AuthUseCase, cfg WebBFFConfig) Handler {
	mw := nitroadapter.WebBFFLoginHandler(auth, cfg)
	return wrapAdapter(mw)
}

// WebBFFRegisterHandler returns a web BFF register handler.
func WebBFFRegisterHandler(auth *AuthUseCase, cfg WebBFFConfig) Handler {
	mw := nitroadapter.WebBFFRegisterHandler(auth, cfg)
	return wrapAdapter(mw)
}

// WebBFFRefreshHandler returns a web BFF refresh handler.
func WebBFFRefreshHandler(auth *AuthUseCase, cfg WebBFFConfig) Handler {
	mw := nitroadapter.WebBFFRefreshHandler(auth, cfg)
	return wrapAdapter(mw)
}

// WebBFFLogoutHandler returns a web BFF logout handler.
func WebBFFLogoutHandler(auth *AuthUseCase, cfg WebBFFConfig) Handler {
	mw := nitroadapter.WebBFFLogoutHandler(auth, cfg)
	return wrapAdapter(mw)
}

// CSRFTokenHandler returns a handler that generates CSRF tokens.
func CSRFTokenHandler(cfg WebBFFConfig) Handler {
	mw := nitroadapter.CSRFTokenHandler(cfg)
	return wrapAdapter(mw)
}

// CSRFMiddleware returns CSRF protection middleware.
func CSRFMiddleware(cfg WebBFFConfig) Handler {
	mw := nitroadapter.CSRFMiddleware(cfg)
	return wrapAdapter(mw)
}

// SetTrustedProxies configures which peer IPs may set X-Forwarded-For / X-Real-IP
// for ClientIP(). Empty (default) never trusts proxy headers.
func SetTrustedProxies(cidrs ...string) error {
	return nitroadapter.SetTrustedProxies(cidrs...)
}

// === Standalone CSRF ===

// CSRFConfig is configuration for standalone CSRF.
type CSRFConfig = nitroadapter.CSRFConfig

// StandaloneCSRFToken returns a handler that generates CSRF tokens.
func StandaloneCSRFToken(cfg ...CSRFConfig) Handler {
	mw := nitroadapter.StandaloneCSRFToken(cfg...)
	return wrapAdapter(mw)
}

// StandaloneCSRFMiddleware returns CSRF protection middleware.
func StandaloneCSRFMiddleware(cfg ...CSRFConfig) Handler {
	mw := nitroadapter.StandaloneCSRFMiddleware(cfg...)
	return wrapAdapter(mw)
}

// === Mobile BFF Types ===

// MobileBFFConfig configures the BFF for mobile applications.
type MobileBFFConfig = nitroadapter.MobileBFFConfig

// DeviceStore persists mobile devices (EnableDeviceTracking). Plug a database
// implementation for several replicas; the default is in-memory.
type DeviceStore = nitroadapter.DeviceStore

// NewMemoryDeviceStore creates an in-memory DeviceStore keeping at most
// maxPerUser devices per user (least recently seen evicted; 0 = unlimited).
func NewMemoryDeviceStore(maxPerUser int) *nitroadapter.MemoryDeviceStore {
	return nitroadapter.NewMemoryDeviceStore(maxPerUser)
}

// === Mobile BFF Handlers ===

// MobileBFFMiddleware returns BFF middleware for mobile.
func MobileBFFMiddleware(cfg MobileBFFConfig, tokenProvider ports.TokenProvider) Handler {
	mw := nitroadapter.MobileBFFMiddleware(cfg, tokenProvider)
	return wrapAdapter(mw)
}

// MobileBFFLoginHandler returns a mobile BFF login handler.
func MobileBFFLoginHandler(auth *AuthUseCase, cfg MobileBFFConfig) Handler {
	mw := nitroadapter.MobileBFFLoginHandler(auth, cfg)
	return wrapAdapter(mw)
}

// MobileBFFRegisterHandler returns a mobile BFF register handler.
func MobileBFFRegisterHandler(auth *AuthUseCase, cfg MobileBFFConfig) Handler {
	mw := nitroadapter.MobileBFFRegisterHandler(auth, cfg)
	return wrapAdapter(mw)
}

// MobileBFFRefreshHandler returns a mobile BFF refresh handler.
func MobileBFFRefreshHandler(auth *AuthUseCase, cfg MobileBFFConfig) Handler {
	mw := nitroadapter.MobileBFFRefreshHandler(auth, cfg)
	return wrapAdapter(mw)
}

// MobileBFFLogoutHandler returns a mobile BFF logout handler.
func MobileBFFLogoutHandler(auth *AuthUseCase, cfg MobileBFFConfig) Handler {
	mw := nitroadapter.MobileBFFLogoutHandler(auth, cfg)
	return wrapAdapter(mw)
}

// === BFF Route Registration ===

// WebBFFRoutes registers all web BFF routes.
func (a *App) WebBFFRoutes(prefix string, auth *AuthUseCase, cfg WebBFFConfig, tokenProvider ports.TokenProvider) *App {
	nitroadapter.RegisterWebBFFRoutes(a.inner, prefix, auth, cfg, tokenProvider)
	return a
}

// MobileBFFRoutes registers all mobile BFF routes.
func (a *App) MobileBFFRoutes(prefix string, auth *AuthUseCase, cfg MobileBFFConfig, tokenProvider ports.TokenProvider) *App {
	nitroadapter.RegisterMobileBFFRoutes(a.inner, prefix, auth, cfg, tokenProvider)
	return a
}

// === BFF Config Constructors ===

// NewWebBFFConfig creates a web BFF configuration.
func NewWebBFFConfig(services ...ServiceConfig) WebBFFConfig {
	cfg := WebBFFConfig{
		BFFConfig: BFFConfig{
			Name:     "web",
			Services: services,
		},
		TokenCookieName:   "_at",
		RefreshCookieName: "_rt",
		CSRFCookieName:    "_csrf",
		CookiePath:        "/",
		CookieSecure:      true,
		CookieSameSite:    "Lax",
		CookieHTTPOnly:    true,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   7 * 24 * time.Hour,
		EnableCSRF:        true,
	}
	return cfg
}

// NewMobileBFFConfig creates a mobile BFF configuration.
func NewMobileBFFConfig(services ...ServiceConfig) MobileBFFConfig {
	cfg := MobileBFFConfig{
		BFFConfig: BFFConfig{
			Name:     "mobile",
			Services: services,
		},
		TokenHeader:          "Authorization",
		TokenPrefix:          "Bearer ",
		AccessTokenTTL:       30 * time.Minute,
		RefreshTokenTTL:      30 * 24 * time.Hour,
		EnableDeviceTracking: true,
		MaxDevicesPerUser:    5,
		Devices:              nitroadapter.NewMemoryDeviceStore(5),
		CompactResponse:      true,
	}
	return cfg
}
