package nitro

import (
	"errors"
	"fmt"
	"os"
	"time"

	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	oteladapter "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// DefaultMaxBody is the default request body limit used by Defaults (1 MiB).
const DefaultMaxBody int64 = 1 << 20

// MaxBodySize limits request body size (413 when exceeded).
func MaxBodySize(maxBytes int64) Handler {
	mw := nitroadapter.MaxBodySizeMiddleware(maxBytes)
	return func(c *Ctx) error {
		return mw(&c.Ctx)
	}
}

// SecurityHeaders returns the security headers middleware.
func SecurityHeaders(cfg ...SecurityConfig) Handler {
	var mw domain.Handler
	if len(cfg) > 0 {
		mw = nitroadapter.SecurityHeadersWithConfig(cfg[0])
	} else {
		mw = nitroadapter.SecurityHeadersMiddleware()
	}
	return func(c *Ctx) error {
		return mw(&c.Ctx)
	}
}

// Defaults registers Recover + Logger + MaxBody + OpenTelemetry HTTP (idempotent).
// In development (Config.Env == EnvDevelopment), also enables permissive CORS.
// Also applies Config.Security.CORS when AllowedOrigins is set.
//
// nitro.New() already calls Defaults() — only call again if you used
// Config.DisableDefaults.
//
// Telemetry opt-out: Config.DisableTelemetry, NITRO_OTEL=0, OTEL_SDK_DISABLED=true.
func (a *App) Defaults() *App {
	return a.applyDefaults()
}

func (a *App) applyDefaults() *App {
	if a.defaultsApplied {
		return a
	}
	a.defaultsApplied = true

	a.inner.Use(nitroadapter.RecoverMiddlewareWithDebug(a.cfg.Debug))
	pre, post := nitroadapter.Logger(nitroadapter.LoggerConfig{
		SkipRoutes: nitroadapter.DefaultSkipRoutes,
		Colors:     loggerColorsForEnv(a.cfg.Env),
	})
	a.inner.Use(pre)
	a.inner.UsePost(post)
	a.inner.Use(nitroadapter.MaxBodySizeMiddleware(DefaultMaxBody))

	if !a.cfg.DisableTelemetry && !oteladapter.Disabled() {
		_ = oteladapter.Ensure(a.cfg.ServiceName)
		tPre, tPost := oteladapter.TracingPair(domain.DefaultTracer())
		mPre, mPost := oteladapter.MetricsPair(domain.DefaultMeter())
		a.inner.Use(tPre)
		a.inner.Use(mPre)
		a.inner.UsePost(mPost)
		a.inner.UsePost(tPost)
	} else if a.cfg.DisableTelemetry {
		oteladapter.ForceDisable()
	}

	if a.cfg.Env == EnvDevelopment {
		a.inner.Use(nitroadapter.CORSMiddleware(domain.CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			// Reflect preflight headers (*): Authorization + X-Tenant-ID / X-Tenant-Slug etc.
			AllowedHeaders: []string{"*"},
		}))
	} else if len(a.cfg.Security.CORS.AllowedOrigins) > 0 {
		a.inner.Use(nitroadapter.CORSMiddleware(a.cfg.Security.CORS))
	}
	return a
}

func loggerColorsForEnv(env domain.Env) string {
	if env == EnvDevelopment {
		return "always"
	}
	return "auto"
}

// Stack is a reusable middleware bundle applied via UseStack.
type Stack func(*App)

// APIStack = Defaults + RateLimit + Compress (post).
var APIStack Stack = func(a *App) {
	a.Defaults()
	a.Use(RateLimit(100, time.Minute))
	a.UsePost(CompressMiddleware())
}

// PublicStack = Defaults + permissive CORS (always, including production).
// Prefer explicit AllowedOrigins in production. Booting with
// Env == EnvProduction fails fast (Listen returns an error) unless
// Config.Security.AllowWildcardCORSInProd is explicitly set to true —
// silently allowing CORS "*" in production was previously only a stderr
// warning that did not stop the boot. See vuln.md (V-14).
var PublicStack Stack = func(a *App) {
	a.Defaults()
	if a.cfg.Env == EnvProduction {
		if !a.cfg.Security.AllowWildcardCORSInProd {
			a.fail(errors.New("nitro: PublicStack enables CORS AllowedOrigins=* in production; set Config.Security.AllowWildcardCORSInProd=true to acknowledge this, or use nitro.CORSMiddleware with explicit AllowedOrigins instead"))
			return
		}
		fmt.Fprintln(os.Stderr, "nitro: warning: PublicStack enables CORS AllowedOrigins=* in production; prefer explicit origins")
	}
	a.Use(CORSMiddleware(CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Tenant-ID", "X-Tenant-Slug"},
	}))
}

// UseStack applies one or more middleware stacks.
func (a *App) UseStack(stacks ...Stack) *App {
	for _, s := range stacks {
		if s != nil {
			s(a)
		}
	}
	return a
}
