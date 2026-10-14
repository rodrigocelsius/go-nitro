package fasthttp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// === BFF Config (base for Web and Mobile) ===

// BFFConfig is the base configuration for BFF.
type BFFConfig struct {
	// Identification
	Name    string `json:"name"`    // "web", "mobile", "desktop"
	Version string `json:"version"` // "1.0.0"

	// Upstream services
	Services []ServiceConfig `json:"services"`

	// Auth
	AuthConfig domain.AuthConfig `json:"auth"`

	// Cache
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"` // 5min

	// Rate Limiting
	RateLimitEnabled bool `json:"rate_limit_enabled"`
	RateLimitMax     int  `json:"rate_limit_max"` // 100 req/min

	// Logging
	LogRequests bool `json:"log_requests"`
}

// ServiceConfig configures an upstream service.
type ServiceConfig struct {
	Name    string            `json:"name"`     // "users", "orders", "products"
	BaseURL string            `json:"base_url"` // "http://users-svc:8081"
	Timeout time.Duration     `json:"timeout"`  // 5s
	Headers map[string]string `json:"headers"`  // fixed headers
}

// === BFF Route ===

// BFFRoute defines a route that the BFF proxies to a service.
type BFFRoute struct {
	Method    string                           // GET, POST, PUT, DELETE
	Path      string                           // /users, /orders/:id
	Service   string                           // "users" (service name)
	Upstream  string                           // /api/users (upstream path)
	Auth      bool                             // requires authentication?
	Cache     time.Duration                    // cache duration (0 = no cache)
	Transform func(*domain.Ctx, []byte) []byte // optional transformation
}

// === BFF Proxy Handler ===

// BFFProxyHandler returns a handler that proxies requests to upstream services.
//
// Usage:
//
//	app.Get("/users", nitro.BFFProxyHandler(bffConfig, "users", "/api/users"))
//	app.Get("/users/:id", nitro.BFFProxyHandler(bffConfig, "users", "/api/users/:id"))
func BFFProxyHandler(cfg BFFConfig, serviceName, upstreamPath string) domain.Handler {
	h, err := NewBFFProxyHandler(cfg, serviceName, upstreamPath)
	if err != nil {
		panic(err) // Must-style: a typo'd service name is a programmer error at startup
	}
	return h
}

// NewBFFProxyHandler is BFFProxyHandler returning an error instead of panicking
// when serviceName is not configured.
func NewBFFProxyHandler(cfg BFFConfig, serviceName, upstreamPath string) (domain.Handler, error) {
	service := findService(cfg.Services, serviceName)
	if service == nil {
		return nil, fmt.Errorf("bff: service not found: %s", serviceName)
	}

	client := &http.Client{Timeout: service.Timeout}
	// One cache per handler: entries must never be shared across routes or
	// upstream services.
	var respCache *bffCache
	if cfg.CacheEnabled {
		respCache = newBFFCache(bffCacheMaxEntries, bffCacheMaxEntryBytes)
	}

	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		// 1. Build upstream URL
		url := service.BaseURL + resolveUpstreamPath(upstreamPath, c)

		// 2. Extract relevant headers
		headers := extractHeaders(fc, service.Headers)

		// 3. Check cache (if enabled)
		var cacheKey string
		if respCache != nil && fc.Method() == "GET" {
			cacheKey = buildCacheKey(fc, serviceName, url, headers)
			if cached, ok := respCache.get(cacheKey); ok {
				return fc.Write(200, cached)
			}
		}

		// 4. Make upstream request
		resp, err := proxyRequest(client, fc.Method(), url, fc.Body(), headers)
		if err != nil {
			return normalizeError(err)
		}
		defer resp.Body.Close()

		// Bound what we buffer from the upstream: an unbounded ReadAll lets a
		// slow/huge or compromised upstream exhaust the gateway's memory.
		body, err := io.ReadAll(io.LimitReader(resp.Body, bffMaxUpstreamBody+1))
		if err != nil {
			return domain.NewInternal(err)
		}
		if int64(len(body)) > bffMaxUpstreamBody {
			return domain.NewInternal(errors.New("bff: upstream response too large"))
		}

		// 5. Apply transformation (if defined)
		if transform := getTransform(fc); transform != nil {
			body = transform(c, body)
		}

		// 6. Store in cache (if enabled)
		if respCache != nil && cacheKey != "" && resp.StatusCode == 200 && bffResponseCacheable(resp.Header) {
			ttl := cfg.CacheTTL
			if ttl <= 0 {
				ttl = 5 * time.Minute
			}
			respCache.set(cacheKey, body, ttl)
		}

		// 7. Return normalized response
		return fc.Write(resp.StatusCode, body)
	}, nil
}

// === BFF Aggregator Handler ===

// BFFAggregateHandler returns a handler that aggregates multiple requests.
//
// Usage:
//
//	app.Get("/dashboard", nitro.BFFAggregateHandler(bffConfig, []AggregateRequest{
//	    {Service: "users", Path: "/api/users/me", Key: "user"},
//	    {Service: "orders", Path: "/api/orders?limit=5", Key: "recent_orders"},
//	    {Service: "notifications", Path: "/api/notifications/unread", Key: "unread"},
//	}))
func BFFAggregateHandler(cfg BFFConfig, requests []AggregateRequest) domain.Handler {
	client := &http.Client{Timeout: 10 * time.Second}

	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		results := make(map[string]any)
		errors := make(map[string]string)

		// Execute requests in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, req := range requests {
			wg.Add(1)
			go func(r AggregateRequest) {
				defer wg.Done()

				service := findService(cfg.Services, r.Service)
				if service == nil {
					mu.Lock()
					errors[r.Key] = "service not found: " + r.Service
					mu.Unlock()
					return
				}

				url := service.BaseURL + r.Path
				method := r.Method
				if method == "" {
					method = "GET"
				}

				resp, err := proxyRequest(client, method, url, r.Body, nil)
				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					errors[r.Key] = err.Error()
				} else {
					defer resp.Body.Close()
					body, _ := io.ReadAll(resp.Body)
					var data any
					json.Unmarshal(body, &data)
					results[r.Key] = data
				}
			}(req)
		}

		wg.Wait()

		// Return aggregated result
		response := map[string]any{
			"data": results,
		}
		if len(errors) > 0 {
			response["errors"] = errors
		}

		return fc.JSON(200, response)
	}
}

// AggregateRequest defines a request for aggregation.
type AggregateRequest struct {
	Service string // service name
	Path    string // upstream path
	Key     string // key in result
	Method  string // GET, POST (default: GET)
	Body    []byte // body for POST
}

// === BFF Transform ===

// TransformFunc defines a response transformation function.
type TransformFunc func(c *domain.Ctx, body []byte) []byte

// TransformRegistry registers transformations per route.
type TransformRegistry struct {
	transforms map[string]TransformFunc
}

// NewTransformRegistry creates a new transform registry.
func NewTransformRegistry() *TransformRegistry {
	return &TransformRegistry{
		transforms: make(map[string]TransformFunc),
	}
}

// Register registers a transformation for a method+path.
func (r *TransformRegistry) Register(method, path string, fn TransformFunc) {
	key := method + ":" + path
	r.transforms[key] = fn
}

// Get returns the transformation for a method+path.
func (r *TransformRegistry) Get(method, path string) TransformFunc {
	key := method + ":" + path
	return r.transforms[key]
}

// === Helper Functions ===

func getFastCtx(c *domain.Ctx) *FastCtx {
	if fc, ok := c.AdapterData.(*FastCtx); ok {
		return fc
	}
	return nil
}

func findService(services []ServiceConfig, name string) *ServiceConfig {
	for i := range services {
		if services[i].Name == name {
			return &services[i]
		}
	}
	return nil
}

// resolveUpstreamPath substitutes :param placeholders in upstreamPath with
// the matched route parameter values. Values are URL-escaped (PathEscape)
// before substitution — without it, a raw client-controlled value like
// "../admin" or "x?evil=1" injected via a route param (e.g. GET
// /api/orders/:id proxied to upstream "/orders/:id") could break out of
// the intended path segment or smuggle extra query parameters into the
// upstream request (CWE-22 / request-parameter injection through the BFF).
func resolveUpstreamPath(upstreamPath string, c *domain.Ctx) string {
	result := upstreamPath
	for i := 0; i < c.ParamCount; i++ {
		paramName := c.ParamNames[i]
		paramValue := url.PathEscape(c.ParamValues[i])
		result = strings.Replace(result, ":"+paramName, paramValue, -1)
	}
	return result
}

func extractHeaders(fc *FastCtx, fixedHeaders map[string]string) map[string]string {
	headers := make(map[string]string)
	// Add fixed headers
	for k, v := range fixedHeaders {
		headers[k] = v
	}
	// Add authorization header if present
	if auth := fc.Header("Authorization"); auth != "" {
		headers["Authorization"] = auth
	}
	// Add content type
	if ct := fc.Header("Content-Type"); ct != "" {
		headers["Content-Type"] = ct
	}
	return headers
}

func proxyRequest(client *http.Client, method, url string, body []byte, headers map[string]string) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, reqBody)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return client.Do(req)
}

func normalizeError(err error) error {
	return domain.NewInternal(err)
}

// bffMaxUpstreamBody caps how much of an upstream response is buffered.
const bffMaxUpstreamBody int64 = 10 << 20 // 10 MiB

const (
	bffCacheMaxEntries    = 1000
	bffCacheMaxEntryBytes = 1 << 20 // responses above 1 MiB are not cached
)

// buildCacheKey identifies a cacheable response. It includes everything that
// can change the upstream answer for a given caller — service, resolved
// upstream URL, query string, the caller's credentials (hashed) and tenant —
// so one user's response can never be served to another.
func buildCacheKey(fc *FastCtx, service, upstreamURL string, headers map[string]string) string {
	h := sha256.New()
	h.Write([]byte(headers["Authorization"]))
	h.Write([]byte{0})
	h.Write([]byte(fc.TenantID()))
	return fc.Method() + "\x00" + service + "\x00" + upstreamURL + "\x00" +
		string(fc.FasthttpCtx().QueryArgs().QueryString()) + "\x00" + hex.EncodeToString(h.Sum(nil))
}

// bffResponseCacheable refuses to cache responses the upstream marked as
// user-specific or that set cookies.
func bffResponseCacheable(h http.Header) bool {
	if h.Get("Set-Cookie") != "" {
		return false
	}
	cc := strings.ToLower(h.Get("Cache-Control"))
	return !(strings.Contains(cc, "private") || strings.Contains(cc, "no-store") || strings.Contains(cc, "no-cache"))
}

// === Bounded In-Memory Cache ===

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

type bffCache struct {
	mu         sync.Mutex
	entries    map[string]*cacheEntry
	maxEntries int
	maxBytes   int
}

func newBFFCache(maxEntries, maxEntryBytes int) *bffCache {
	return &bffCache{entries: make(map[string]*cacheEntry), maxEntries: maxEntries, maxBytes: maxEntryBytes}
}

func (c *bffCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.data, true
}

func (c *bffCache) set(key string, data []byte, ttl time.Duration) {
	if len(data) > c.maxBytes {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
		for k, e := range c.entries { // drop expired entries first
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		for k := range c.entries { // still full: evict one arbitrary entry
			if len(c.entries) < c.maxEntries {
				break
			}
			delete(c.entries, k)
		}
	}
	c.entries[key] = &cacheEntry{data: append([]byte(nil), data...), expiresAt: now.Add(ttl)}
}

// === Transform Helpers ===

func getTransform(fc *FastCtx) TransformFunc {
	// For now, return nil. In the future, we can store transforms in context
	return nil
}
