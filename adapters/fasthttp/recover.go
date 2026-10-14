package fasthttp

import (
	"runtime/debug"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// RecoverMiddleware returns middleware that adds panic recovery with
// stack traces in development mode. Panic recovery is always active
// at the server level; this middleware provides environment-awareness.
func RecoverMiddleware() domain.Handler {
	return recoverMiddleware(false)
}

// RecoverMiddlewareWithEnv returns middleware with environment-aware panic recovery.
// In development mode, stack traces are included in responses.
//
// Deprecated: Env also gates CORS/HSTS and other production hardening —
// a staging/internal deployment left at Env=EnvDevelopment for those
// reasons would unintentionally also leak stack traces to any client
// that can reach it. Prefer RecoverMiddlewareWithDebug with an explicit,
// separate Config.Debug flag (see vuln.md V-33).
func RecoverMiddlewareWithEnv(env domain.Env) domain.Handler {
	return recoverMiddleware(env == domain.EnvDevelopment)
}

// RecoverMiddlewareWithDebug returns middleware whose stack-trace exposure
// is controlled by an explicit debug flag, decoupled from Env.
func RecoverMiddlewareWithDebug(debugMode bool) domain.Handler {
	return recoverMiddleware(debugMode)
}

func recoverMiddleware(debugMode bool) domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()

		if debugMode && fctx.Response.StatusCode() == fasthttp.StatusInternalServerError {
			stack := debug.Stack()
			// Sanitize stack trace: remove file paths and internal details
			// Only show function names, not full paths that could leak system info
			sanitized := sanitizeStackTrace(string(stack))
			msg := "panic stack trace:\n" + sanitized
			fctx.Response.SetBodyString(msg)
		}
		return nil
	}
}

// sanitizeStackTrace removes sensitive information from stack traces.
// In development mode, it still shows useful debugging info but removes
// full file paths that could leak system structure.
func sanitizeStackTrace(stack string) string {
	lines := strings.Split(stack, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Keep goroutine header and function names
		if strings.HasPrefix(trimmed, "goroutine") ||
			strings.HasPrefix(trimmed, "runtime.") ||
			strings.HasPrefix(trimmed, "github.com/rodrigocelsius/go-nitro/") {
			result = append(result, line)
		} else if strings.Contains(line, ".go:") {
			// For file references, only show the filename, not full path
			parts := strings.Split(line, "/")
			if len(parts) > 0 {
				result = append(result, "  "+parts[len(parts)-1])
			}
		}
	}
	if len(result) == 0 {
		return "(stack trace filtered)"
	}
	return strings.Join(result, "\n")
}
