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

	// Push notifications
	EnablePushNotifications bool   `json:"enable_push_notifications"`
	FCMServerKey            string `json:"fcm_server_key"`
	APNSKeyID               string `json:"apns_key_id"`
	APNSPrivateKey          string `json:"apns_private_key"`

	// Biometric auth
	EnableBiometric bool `json:"enable_biometric"`

	// Offline support
	EnableOffline   bool          `json:"enable_offline"`
	OfflineCacheTTL time.Duration `json:"offline_cache_ttl"` // 24h

	// Mobile-specific
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
			deviceID := fc.Header("X-Device-ID")
			if deviceID != "" {
				go updateDeviceLastSeen(context.Background(), deviceID)
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
			req.Device.UserID = "" // Will be set from token
			req.Device.LastSeenAt = time.Now()
			if req.Device.CreatedAt.IsZero() {
				req.Device.CreatedAt = time.Now()
			}
			go saveDevice(context.Background(), &req.Device)
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
			req.Device.LastSeenAt = time.Now()
			if req.Device.CreatedAt.IsZero() {
				req.Device.CreatedAt = time.Now()
			}
			go saveDevice(context.Background(), &req.Device)
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
			go updateDeviceLastSeen(context.Background(), req.DeviceID)
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
			go deleteDevice(context.Background(), req.DeviceID)
		}

		return fc.JSON(200, map[string]string{"message": "logged out"})
	}
}

// === Helper Functions ===

// updateDeviceLastSeen updates the last seen time for a device.
func updateDeviceLastSeen(ctx context.Context, deviceID string) {
	// In production, this would update the device in the database
	// For now, it's a no-op placeholder
	_ = ctx
	_ = deviceID
}

// saveDevice saves a device record.
func saveDevice(ctx context.Context, device *domain.DeviceInfo) {
	// In production, this would save to the database
	// For now, it's a no-op placeholder
	_ = ctx
	_ = device
}

// deleteDevice deletes a device record.
func deleteDevice(ctx context.Context, deviceID string) {
	// In production, this would delete from the database
	// For now, it's a no-op placeholder
	_ = ctx
	_ = deviceID
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
		// These would be implemented with actual device management logic
		// protected.Post("/device", MobileRegisterDeviceHandler(cfg))
		// protected.Delete("/device/:id", MobileRemoveDeviceHandler(cfg))
		// protected.Get("/devices", MobileListDevicesHandler(cfg))
	}

	// Push notification routes (if enabled)
	if cfg.EnablePushNotifications {
		// These would be implemented with actual push notification logic
		// protected.Post("/push/register", MobilePushRegisterHandler(cfg))
		// protected.Post("/push/test", MobilePushTestHandler(cfg))
	}

	// Example: proxy routes would be registered here
	// protected.Get("/users", BFFProxyHandler(cfg.BFFConfig, "users", "/api/users"))
}
