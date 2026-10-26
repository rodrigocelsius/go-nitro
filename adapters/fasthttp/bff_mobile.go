package fasthttp

import (
	"context"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// MobileBFFConfig configures the BFF for mobile applications.
type MobileBFFConfig struct {
	BFFConfig // embeds base config

	// Token storage (in body, app stores in SecureStorage)
	TokenHeader string `json:"token_header"` // "Authorization"
	TokenPrefix string `json:"token_prefix"` // "Bearer "

	// Token TTL (longer than web)
	AccessTokenTTL  time.Duration `json:"access_token_ttl"`  // 30min
	RefreshTokenTTL time.Duration `json:"refresh_token_ttl"` // 30d

	// Device management
	EnableDeviceTracking bool `json:"enable_device_tracking"`
	MaxDevicesPerUser    int  `json:"max_devices_per_user"` // 5
	// Devices persists devices (nil → process-wide in-memory store).
	Devices DeviceStore `json:"-"`

	// Push notifications: device FCM/APNs tokens are stored with the device
	// (DeviceStore); sending pushes is up to the application.
	EnablePushNotifications bool   `json:"enable_push_notifications"`
	FCMServerKey            string `json:"fcm_server_key"`
	APNSKeyID               string `json:"apns_key_id"`
	APNSPrivateKey          string `json:"apns_private_key"`

	// Biometric auth
	// Reserved: no built-in behavior yet (implement in the app).
	EnableBiometric bool `json:"enable_biometric"`

	// Offline support
	// Reserved: no built-in behavior yet (implement in the app).
	EnableOffline   bool          `json:"enable_offline"`
	OfflineCacheTTL time.Duration `json:"offline_cache_ttl"` // 24h

	// Mobile-specific
	// Reserved: no built-in behavior yet (responses are already compact JSON).
	CompactResponse bool `json:"compact_response"` // compact JSON
	DeltaSync       bool `json:"delta_sync"`       // delta synchronization
}

// === Mobile BFF Middleware ===

// MobileBFFMiddleware returns a middleware that:
// 1. Reads token from Authorization header
// 2. Validates the token
// 3. Injects data into context
// 4. Updates device last seen (if tracking enabled)
// 5. Injects headers for downstream services
//
// Usage:
//
//	mobile := app.Group("/mobile", nitro.MobileBFFMiddleware(mobileConfig))
//	mobile.Get("/users", nitro.BFFProxyHandler(mobileConfig, "users", "/api/users"))
func MobileBFFMiddleware(cfg MobileBFFConfig, tokenProvider ports.TokenProvider) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		// 1. Read token from header
		auth := fc.Header("Authorization")
		if auth == "" {
			return domain.NewUnauthorized("missing authorization")
		}

		// 2. Validate format "Bearer <token>"
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return domain.NewUnauthorized("invalid authorization format")
		}

		// 3. Validate token
		claims, err := tokenProvider.ValidateToken(c.Context(), parts[1])
		if err != nil {
			return domain.NewUnauthorized("invalid token")
		}

		// 4. Inject data into context
		fc.Set("user_id", claims.UserID)
		fc.Set("email", claims.Email)
		fc.Set("roles", claims.Roles)
		fc.Set("claims", claims)
		if claims.TenantID != "" {
			fc.Set("tenant_id", claims.TenantID)
		}

		// 5. Update device last seen (if tracking enabled)
		if cfg.EnableDeviceTracking {
			if deviceID := fc.Header("X-Device-ID"); deviceID != "" {
				logDeviceErr("touch", deviceStore(cfg).Touch(c.Context(), deviceID, claims.UserID))
			}
		}

		// 6. Inject headers on the REQUEST for downstream proxy (never on response)
		fc.SetRequestHeader("Authorization", "Bearer "+parts[1])
		fc.SetRequestHeader("X-Forwarded-By", "nitro-mobile-bff")
		fc.SetRequestHeader("X-User-ID", claims.UserID)
		fc.SetRequestHeader("X-Client-Type", "mobile")

		return nil
	}
}

// === Mobile BFF Login Handler ===

// MobileBFFLoginHandler returns a handler that handles mobile login.
// Returns tokens in response body (app stores in Keychain/Keystore).
func MobileBFFLoginHandler(auth *usecases.AuthUseCase, cfg MobileBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		var req struct {
			Email    string            `json:"email"`
			Password string            `json:"password"`
			Device   domain.DeviceInfo `json:"device"`
		}

		if err := fc.BindJSON(&req); err != nil {
			return domain.NewBadRequest("invalid request")
		}

		tokens, err := auth.Login(c.Context(), req.Email, req.Password)
		if err != nil {
			return err
		}

		// Register device (if tracking enabled)
		if cfg.EnableDeviceTracking && req.Device.DeviceID != "" {
			registerDevice(c.Context(), auth, cfg, req.Device, tokens.AccessToken)
		}

		// Response WITH tokens (app stores in Keychain/Keystore)
		return fc.JSON(200, map[string]any{
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
			"token_type":    tokens.TokenType,
		})
	}
}

// === Mobile BFF Register Handler ===

// MobileBFFRegisterHandler returns a handler that handles mobile registration.
func MobileBFFRegisterHandler(auth *usecases.AuthUseCase, cfg MobileBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		var req struct {
			Email    string            `json:"email"`
			Password string            `json:"password"`
			Name     string            `json:"name"`
			Device   domain.DeviceInfo `json:"device"`
		}

		if err := fc.BindJSON(&req); err != nil {
			return domain.NewBadRequest("invalid request")
		}

		tokens, err := auth.Register(c.Context(), req.Email, req.Password, req.Name)
		if err != nil {
			return err
		}

		// Register device (if tracking enabled)
		if cfg.EnableDeviceTracking && req.Device.DeviceID != "" {
			registerDevice(c.Context(), auth, cfg, req.Device, tokens.AccessToken)
		}

		return fc.JSON(201, map[string]any{
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
			"token_type":    tokens.TokenType,
		})
	}
}

// === Mobile BFF Refresh Handler ===

// MobileBFFRefreshHandler returns a handler that refreshes mobile tokens.
func MobileBFFRefreshHandler(auth *usecases.AuthUseCase, cfg MobileBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		var req struct {
			RefreshToken string `json:"refresh_token"`
			DeviceID     string `json:"device_id"`
		}

		if err := fc.BindJSON(&req); err != nil {
			return domain.NewBadRequest("invalid request")
		}

		tokens, err := auth.RefreshToken(c.Context(), req.RefreshToken)
		if err != nil {
			return domain.NewUnauthorized("session expired")
		}

		// Update device last seen (if tracking enabled)
		if cfg.EnableDeviceTracking && req.DeviceID != "" {
			userID := ""
			if cl, err := auth.Claims(c.Context(), tokens.AccessToken); err == nil {
				userID = cl.UserID
			}
			logDeviceErr("touch", deviceStore(cfg).Touch(c.Context(), req.DeviceID, userID))
		}

		return fc.JSON(200, map[string]any{
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
			"token_type":    tokens.TokenType,
		})
	}
}

// === Mobile BFF Logout Handler ===

// MobileBFFLogoutHandler returns a handler that handles mobile logout.
func MobileBFFLogoutHandler(auth *usecases.AuthUseCase, cfg MobileBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		var req struct {
			RefreshToken string `json:"refresh_token"`
			DeviceID     string `json:"device_id"`
		}
		_ = fc.BindJSON(&req) // Ignore error, fields are optional

		if req.RefreshToken != "" {
			_ = auth.Logout(c.Context(), req.RefreshToken)
		}

		// Delete device (if tracking enabled)
		if cfg.EnableDeviceTracking && req.DeviceID != "" {
			logDeviceErr("delete", deviceStore(cfg).Delete(c.Context(), req.DeviceID))
		}

		return fc.JSON(200, map[string]string{"message": "logged out"})
	}
}

// registerDevice stores the device bound to the user of the issued token.
func registerDevice(ctx context.Context, auth *usecases.AuthUseCase, cfg MobileBFFConfig, d domain.DeviceInfo, accessToken string) {
	d.UserID = ""
	if cl, err := auth.Claims(ctx, accessToken); err == nil {
		d.UserID = cl.UserID
	}
	d.LastSeenAt = time.Now()
	logDeviceErr("save", deviceStore(cfg).Save(ctx, d))
}

// === Route Registration Helpers ===

// RegisterMobileBFFRoutes registers standard mobile BFF routes.
func RegisterMobileBFFRoutes(app *App, prefix string, auth *usecases.AuthUseCase, cfg MobileBFFConfig, tokenProvider ports.TokenProvider) {
	// Auth routes (public)
	app.Post(prefix+"/auth/login", MobileBFFLoginHandler(auth, cfg))
	app.Post(prefix+"/auth/register", MobileBFFRegisterHandler(auth, cfg))
	app.Post(prefix+"/auth/refresh", MobileBFFRefreshHandler(auth, cfg))
	app.Post(prefix+"/auth/logout", MobileBFFLogoutHandler(auth, cfg))

	// Protected routes with BFF middleware
	protected := app.Group(prefix)
	protected.Use(MobileBFFMiddleware(cfg, tokenProvider))

	// Device management routes (if tracking enabled)
	if cfg.EnableDeviceTracking {
		protected.Get("/devices", MobileListDevicesHandler(cfg))
		protected.Delete("/devices/:id", MobileRemoveDeviceHandler(cfg))
	}

	// Example: proxy routes would be registered here
	// protected.Get("/users", BFFProxyHandler(cfg.BFFConfig, "users", "/api/users"))
}
