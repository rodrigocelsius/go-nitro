// Package fasthttp implements the adapter layer for the Nitro framework,
// providing concrete implementations based on fasthttp.
package fasthttp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/jsontag"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
	"github.com/valyala/fasthttp"
)

// FileHeader contains metadata for an uploaded file.
type FileHeader struct {
	Filename string
	Size     int
	// content stores the file data in memory (populated by FormFile).
	content []byte
}

// ReadAll reads all bytes from the uploaded file.
func (fh *FileHeader) ReadAll() ([]byte, error) {
	return fh.content, nil
}

// SaveFile saves the uploaded file to the given path with restrictive permissions.
//
// path is used as-is — if it embeds fh.Filename (client-controlled),
// combine it with fh.SafeFilename() first, never the raw Filename, to
// avoid path traversal (CWE-22).
func (fh *FileHeader) SaveFile(path string) error {
	return os.WriteFile(path, fh.content, 0600)
}

// SafeFilename returns a filesystem-safe version of Filename: only the
// final path element (filepath.Base after filepath.Clean), with any
// leading dots stripped and a fallback name when that leaves nothing
// usable. The client-supplied Filename is untrusted input — using it
// directly to build a destination path (e.g. filepath.Join(dir,
// fh.Filename)) allows path traversal via "../../etc/passwd" or an
// absolute path that escapes dir (CWE-22). Prefer generating an
// internally-controlled name (e.g. a UUID + extension from
// DetectFileType) over trusting any client-supplied filename, even this
// sanitized one.
func (fh *FileHeader) SafeFilename() string {
	name := filepath.Base(filepath.Clean(fh.Filename))
	name = strings.TrimLeft(name, ".")
	name = strings.TrimSpace(name)
	if name == "" || name == string(filepath.Separator) {
		return "upload"
	}
	return name
}

// DetectFileType sniffs the file's actual content type from its first
// bytes (net/http.DetectContentType, magic-byte based), independent of
// the client-supplied filename extension or multipart Content-Type header
// — both are trivially spoofable and must not be trusted alone to decide
// whether an upload is safe to store/serve (e.g. rejecting an uploaded
// ".jpg" that is actually an HTML/SVG payload). Call before SaveFile.
func DetectFileType(data []byte) string {
	return http.DetectContentType(data)
}

// FastCtx is the concrete request-context implementation,
// combining the pure domain (domain.Ctx) with fasthttp infrastructure.
// domain.Ctx.AdapterData stores the *FastCtx itself so middlewares
// and the facade can recover the concrete context.
type FastCtx struct {
	domain.Ctx
	fasthttpReq *fasthttp.RequestCtx
	startTime   time.Time
	reqCtx      context.Context
	locals      map[string]any
	validator   domain.Validator
	app         *App // set during request handling (reverse routing)
}

// NewFastCtx creates a new FastCtx.
func NewFastCtx() *FastCtx {
	return &FastCtx{}
}

// Reset clears FastCtx for reuse via the pool.
// Stores this FastCtx on AdapterData so middlewares
// and the facade can recover the concrete context.
func (c *FastCtx) Reset(fctx *fasthttp.RequestCtx) {
	c.Ctx.ResetHot()
	c.fasthttpReq = fctx
	c.AdapterData = c
	c.reqCtx = nil
	c.startTime = time.Time{}
	// validator/app are assigned by Handler on each request.
	if c.locals != nil {
		clear(c.locals)
	}
}

// SetValidator associates the App validator with this request context.
func (c *FastCtx) SetValidator(v domain.Validator) {
	c.validator = v
}

// Validator returns the request validator, or the global default.
func (c *FastCtx) Validator() domain.Validator {
	if c.validator != nil {
		return c.validator
	}
	return usecases.DefaultValidator()
}

// FasthttpCtx returns the *fasthttp.RequestCtx associated with this context.
func (c *FastCtx) FasthttpCtx() *fasthttp.RequestCtx {
	return c.fasthttpReq
}

// StartTime returns the request start time.
func (c *FastCtx) StartTime() time.Time {
	return c.startTime
}

// SetStartTime sets the request start time.
func (c *FastCtx) SetStartTime(t time.Time) {
	c.startTime = t
}

// RequestContext returns the request context.Context.
// Detach returns a context.Context that is SAFE to use after the handler
// returns (goroutines, queues, caches, singleflight).
//
// *FastCtx implements context.Context for convenience, but it is recycled
// through a sync.Pool as soon as the handler returns: a retained `c` would
// read another request's data (claims, tenant, deadline) — a cross-user
// leak. Anything that outlives the handler must use c.Detach() (values of
// the request context are kept, cancellation is dropped) and must copy what
// it needs (c.Param(...), c.UserID(), c.TenantID(), body bytes) BEFORE
// starting. Param/Query/Header strings are only valid during the handler.
//
//	uid, tid := c.UserID(), c.TenantID()
//	bg := c.Detach()
//	go func() { _ = svc.Audit(bg, uid, tid) }()
func (c *FastCtx) Detach() context.Context {
	return context.WithoutCancel(c.RequestContext())
}

func (c *FastCtx) RequestContext() context.Context {
	if c.reqCtx != nil {
		return c.reqCtx
	}
	return context.Background()
}

// SetRequestContext sets the request context.Context.
func (c *FastCtx) SetRequestContext(ctx context.Context) {
	c.reqCtx = ctx
}

// Method returns the HTTP request method (GET, POST, etc).
func (c *FastCtx) Method() string {
	return b2s(c.FasthttpCtx().Method())
}

// Path returns the request path.
func (c *FastCtx) Path() string {
	return b2s(c.FasthttpCtx().Path())
}

// Body returns the raw request body.
func (c *FastCtx) Body() []byte {
	return c.FasthttpCtx().PostBody()
}

// SendString writes a string as the response body.
func (c *FastCtx) SendString(s string) error {
	c.FasthttpCtx().SetBodyString(s)
	return nil
}

// Fasthttp exposes the raw *fasthttp.RequestCtx when Nitro's API
// does not yet cover the needed operation.
func (c *FastCtx) Fasthttp() *fasthttp.RequestCtx {
	return c.FasthttpCtx()
}

// jsonMarshaler is implemented by types with generated code.
type jsonMarshaler interface {
	AppendJSON(buf []byte) []byte
}

// jsonUnmarshaler is implemented by types with generated code.
type jsonUnmarshaler interface {
	ParseJSON(data []byte) bool
}

// jsonEncoder bundles a reusable buffer with an encoder bound to it, so
// c.JSON allocates neither a bytes.Buffer nor a json.Encoder per response.
type jsonEncoder struct {
	buf bytes.Buffer
	enc *json.Encoder
}

var jsonEncPool = sync.Pool{
	New: func() any {
		je := &jsonEncoder{}
		je.buf.Grow(512)
		je.enc = json.NewEncoder(&je.buf)
		je.enc.SetEscapeHTML(false)
		return je
	},
}

// maxPooledJSONBuf avoids pinning huge buffers in the pool after a big response.
const maxPooledJSONBuf = 1 << 20

var (
	contentTypeJSON     = []byte("application/json")
	contentTypeTextUTF8 = []byte("text/plain; charset=utf-8")
	contentTypeOctet    = []byte("application/octet-stream")
)

// JSON writes v serialized as JSON in the response body.
// Accepts statusCode + body.
func (c *FastCtx) JSON(statusCode int, v any) error {
	fctx := c.FasthttpCtx()
	fctx.SetStatusCode(statusCode)
	fctx.Response.Header.SetContentTypeBytes(contentTypeJSON)

	if m, ok := v.(jsonMarshaler); ok {
		buf := m.AppendJSON(fctx.Response.Body()[:0])
		fctx.Response.SetBodyRaw(buf)
		return nil
	}

	// Models that omit `json` tags (default snake_case naming — see
	// usecases/jsontag) need a rename pass; fully tagged models (the
	// common case) fall straight into the pooled encoder below, unchanged.
	if jsontag.NeedsRename(v) {
		data, err := jsontag.Marshal(v)
		if err != nil {
			return err
		}
		fctx.SetBody(data)
		return nil
	}

	je := jsonEncPool.Get().(*jsonEncoder)
	je.buf.Reset()
	if err := je.enc.Encode(v); err != nil {
		jsonEncPool.Put(je)
		return err
	}
	data := je.buf.Bytes()
	if n := len(data); n > 0 && data[n-1] == '\n' {
		data = data[:n-1]
	}
	fctx.SetBody(data) // copies into the response buffer
	if je.buf.Cap() <= maxPooledJSONBuf {
		jsonEncPool.Put(je)
	}
	return nil
}

// RequestID returns the unique request identifier.
func (c *FastCtx) RequestID() string {
	return c.Ctx.RequestID()
}

// Log returns the request-scoped logger.
// Does not delegate to c.Ctx.Log() — that would recurse because
// domain.Ctx.Log() falls back to AdapterData.Log() (this method).
func (c *FastCtx) Log() domain.Logger {
	if l := c.Ctx.LoggerOrNil(); l != nil {
		return l
	}
	return defaultAccessLogger
}

// Query returns a query parameter as string.
// Optional second argument: default value.
func (c *FastCtx) Query(key string, defaultVal ...string) string {
	v := b2s(c.FasthttpCtx().QueryArgs().Peek(key))
	if v == "" && len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return v
}

// QueryInt returns a query parameter as int.
func (c *FastCtx) QueryInt(key string) (int, error) {
	v, err := c.FasthttpCtx().QueryArgs().GetUint(key)
	return v, err
}

// QueryIntDefault returns a query parameter as int with a default.
// If the parameter is missing or invalid, returns the default.
func (c *FastCtx) QueryIntDefault(key string, defaultVal int) int {
	v, err := c.FasthttpCtx().QueryArgs().GetUint(key)
	if err != nil {
		return defaultVal
	}
	return v
}

// QueryFloat64 returns a query parameter as float64.
func (c *FastCtx) QueryFloat64(key string) (float64, error) {
	v := c.FasthttpCtx().QueryArgs().Peek(key)
	if len(v) == 0 {
		return 0, nil
	}
	return strconv.ParseFloat(b2s(v), 64)
}

// QueryFloat64Default returns a query parameter as float64 with a default.
// If the parameter is missing or invalid, returns the default.
func (c *FastCtx) QueryFloat64Default(key string, defaultVal float64) float64 {
	v := c.FasthttpCtx().QueryArgs().Peek(key)
	if len(v) == 0 {
		return defaultVal
	}
	f, err := strconv.ParseFloat(b2s(v), 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// QueryBool returns a query parameter as bool.
func (c *FastCtx) QueryBool(key string) bool {
	return parseQueryBool(c.FasthttpCtx().QueryArgs().Peek(key))
}

// Int returns a query parameter as int with a default.
// If the parameter is missing or invalid, returns defaultVal.
func (c *FastCtx) Int(key string, defaultVal int) int {
	v, err := c.FasthttpCtx().QueryArgs().GetUint(key)
	if err != nil {
		return defaultVal
	}
	return v
}

// Float returns a query parameter as float64 with a default.
func (c *FastCtx) Float(key string, defaultVal float64) float64 {
	return c.QueryFloat64Default(key, defaultVal)
}

// Bool returns a query parameter as bool.
// Valores "true", "1", "yes" retornam true.
func (c *FastCtx) Bool(key string) bool {
	return parseQueryBool(c.FasthttpCtx().QueryArgs().Peek(key))
}

// Sort returns the sort field and direction from a query parameter.
// Supports `?sort=-field` for descending order.
//
// Example:
//
//	// GET /users?sort=-created_at
//	field, desc := c.Sort("created_at")
//	// field = "created_at", desc = true
//
//	// GET /users?sort=name
//	field, desc := c.Sort("created_at")
//	// field = "name", desc = false
func (c *FastCtx) Sort(defaultField ...string) (string, bool) {
	sort := c.Query("sort")
	if sort == "" {
		if len(defaultField) > 0 {
			return defaultField[0], false
		}
		return "", false
	}
	if sort[0] == '-' {
		return strings.TrimPrefix(sort, "-"), true
	}
	return sort, false
}

// Header returns a header as string.
// Optional second argument: default value.
func (c *FastCtx) Header(key string, defaultVal ...string) string {
	v := b2s(c.FasthttpCtx().Request.Header.Peek(key))
	if v == "" && len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return v
}

// Cookie returns the cookie value.
func (c *FastCtx) Cookie(name string) string {
	return b2s(c.FasthttpCtx().Request.Header.Cookie(name))
}

// SetCookie sets a cookie on the response.
func (c *FastCtx) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	c.SetCookieWithSameSite(name, value, maxAge, path, domain, secure, httpOnly, fasthttp.CookieSameSiteLaxMode)
}

// SetCookieWithSameSite sets a cookie with an explicit SameSite.
func (c *FastCtx) SetCookieWithSameSite(name, value string, maxAge int, path, domain string, secure, httpOnly bool, sameSite fasthttp.CookieSameSite) {
	var cookie fasthttp.Cookie
	cookie.SetKey(name)
	cookie.SetValue(value)
	cookie.SetMaxAge(maxAge)
	cookie.SetPath(path)
	cookie.SetDomain(domain)
	cookie.SetSecure(secure)
	cookie.SetHTTPOnly(httpOnly)
	cookie.SetSameSite(sameSite)
	c.FasthttpCtx().Response.Header.SetCookie(&cookie)
}

// DeleteCookie removes a cookie by setting MaxAge=-1. Secure is inferred
// from the current request's scheme (like csrf.go/oauth.go already do)
// instead of being hardcoded to false — a deletion cookie sent without
// Secure over HTTPS can be stripped/tampered with by a network attacker
// on a mixed-content path, and some browsers otherwise reject overwriting
// a Secure cookie with a non-Secure one, silently failing the delete.
func (c *FastCtx) DeleteCookie(name string) {
	secure := strings.EqualFold(string(c.FasthttpCtx().URI().Scheme()), "https")
	c.SetCookie(name, "", -1, "/", "", secure, true)
}

// SetStatus sets the HTTP response status code.
func (c *FastCtx) SetStatus(code int) {
	c.FasthttpCtx().SetStatusCode(code)
}

// SetHeader sets a response header.
func (c *FastCtx) SetHeader(key, value string) {
	c.FasthttpCtx().Response.Header.Set(key, value)
}

// SetRequestHeader sets a request header (useful for BFF / proxy).
func (c *FastCtx) SetRequestHeader(key, value string) {
	c.FasthttpCtx().Request.Header.Set(key, value)
}

// SetBody sets the response body.
func (c *FastCtx) SetBody(data []byte) {
	c.FasthttpCtx().SetBody(data)
}

// Bind decodes the request body into v according to Content-Type
// (JSON, form-urlencoded, multipart) — no validation.
// With no Content-Type (or GET), tries JSON if the body looks like JSON; otherwise query.
// For a dynamic map: c.BindMap() or c.Bind(&m) with m map[string]any.
func (c *FastCtx) Bind(v any) error {
	ct := c.contentTypeMedia()
	body := c.FasthttpCtx().PostBody()

	switch {
	case isJSONContentType(ct) || (ct == "" && looksLikeJSON(body)):
		return c.bindJSONRaw(v, body)
	case isFormContentType(ct):
		return c.BindForm(v)
	case ct == "" || len(body) == 0:
		if len(body) > 0 {
			return c.bindJSONRaw(v, body)
		}
		return c.BindQuery(v)
	default:
		if err := c.bindJSONRaw(v, body); err == nil {
			return nil
		}
		return c.BindForm(v)
	}
}

func (c *FastCtx) bindJSONRaw(v any, body []byte) error {
	if u, ok := v.(jsonUnmarshaler); ok {
		if !u.ParseJSON(body) {
			return domain.ErrInvalidJSON{}
		}
		return nil
	}
	return jsontag.Unmarshal(body, v)
}

func (c *FastCtx) contentTypeMedia() string {
	ct := b2s(c.FasthttpCtx().Request.Header.Peek("Content-Type"))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

func isJSONContentType(ct string) bool {
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

func isFormContentType(ct string) bool {
	return ct == "application/x-www-form-urlencoded" ||
		ct == "multipart/form-data" ||
		strings.HasPrefix(ct, "multipart/form-data")
}

func looksLikeJSON(body []byte) bool {
	for _, b := range body {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b == '{' || b == '['
	}
	return false
}

// BindMap decodes the JSON body into map[string]any (no validation / transforms).
// Prefer Parse[T] / BodyJSON for typed structs.
//
//	m, err := c.BindMap()
func (c *FastCtx) BindMap() (map[string]any, error) {
	var m map[string]any
	if err := c.Bind(&m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// BindJSON is deprecated: use BodyJSON (same behavior: bind + validate + 400/422).
// Prefer nitro.Parse[T](c) or c.BodyJSON(&v) in new code. Use c.Bind for decode-only.
func (c *FastCtx) BindJSON(v any) error {
	return c.BodyJSON(v)
}

// BodyParser performs unified bind by Content-Type plus the pipeline
// decode → default → transform → validate (400/422).
// Fiber-like BodyParser equivalent; also available via nitro.Parse[T].
func (c *FastCtx) BodyParser(v any) error {
	ct := c.contentTypeMedia()
	if isJSONContentType(ct) || (ct == "" && looksLikeJSON(c.FasthttpCtx().PostBody())) {
		return c.BodyJSON(v)
	}
	if err := c.Bind(v); err != nil {
		return &domain.BadRequestError{Message: "invalid body"}
	}
	// Form / query: same pipeline as JSON (presence map → defaults → transforms).
	validator.ApplyDefaults(v, c.requestKeyPresence())
	validator.ApplyTransforms(v)
	if errs := c.Validator().Validate(v); errs.HasErrors() {
		return domain.NewValidationFailed(errs)
	}
	return nil
}

// requestKeyPresence builds a map of keys present in query, post args or multipart
// values (for ApplyDefaults after form/query Bind).
func (c *FastCtx) requestKeyPresence() map[string]any {
	m := make(map[string]any)
	fctx := c.FasthttpCtx()
	if fctx == nil {
		return m
	}
	fctx.QueryArgs().VisitAll(func(k, v []byte) {
		m[string(k)] = string(v)
	})
	fctx.PostArgs().VisitAll(func(k, v []byte) {
		m[string(k)] = string(v)
	})
	if mf, err := fctx.MultipartForm(); err == nil && mf != nil {
		for k, vs := range mf.Value {
			if len(vs) > 0 {
				m[k] = vs[0]
			}
		}
	}
	return m
}

// BodyJSON decodes JSON + defaults (`default`) + transforms (`transform`) + validate.
// Pipeline: decode → default → transform → validate.
// Parse failure: 400 with {"error": "invalid json body"}
// Validation failure: 422 with {"error": "validation failed", "fields": {...}}
// Returns nil on success.
//
// Example:
//
//	var req LoginRequest
//	if err := c.BodyJSON(&req); err != nil {
//	    return err  // err is already a complete HTTP response
//	}
func (c *FastCtx) BodyJSON(v any) error {
	body := c.FasthttpCtx().PostBody()
	if err := validator.UnmarshalJSONDefaults(body, v); err != nil {
		return &domain.BadRequestError{Message: "invalid json body"}
	}
	if errs := c.Validator().Validate(v); errs.HasErrors() {
		return domain.NewValidationFailed(errs)
	}
	return nil
}

// ValidationErrorResponse is an alias of domain.ValidationFailedError (compatibility).
type ValidationErrorResponse = domain.ValidationFailedError

// Validate validates the struct using validate tags.
func (c *FastCtx) Validate(v any) domain.ValidationErrors {
	return c.Validator().Validate(v)
}

// FormValue returns a form value by key from URL-encoded or multipart forms.
func (c *FastCtx) FormValue(key string) string {
	return b2s(c.FasthttpCtx().FormValue(key))
}

// ErrFileTooLarge is returned by FormFile/FormFiles when an uploaded
// file exceeds the requested maxSize (see vuln.md V-35).
var ErrFileTooLarge = errors.New("nitro: uploaded file exceeds max size")

// readFilePart reads f's content, optionally capped at maxSize bytes.
// maxSize <= 0 means unlimited (the request's own MaxRequestBodySize /
// MaxBodySizeMiddleware are still the outer bound in that case).
//
// Without a per-file cap, a multipart request under the overall body
// limit can still allocate one huge io.ReadAll buffer per field — e.g.
// several ~1 MiB fields adding up, or a single field sized right under
// the body limit — since FormFile/FormFiles load the whole file into
// memory at once. A caller handling untrusted uploads (e.g. avatars
// capped at 2 MiB) had no way to enforce that per-field, only the
// coarser total-body limit.
func readFilePart(f *multipart.FileHeader, maxSize int64) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if maxSize <= 0 {
		return io.ReadAll(r)
	}
	limited := io.LimitReader(r, maxSize+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxSize {
		return nil, fmt.Errorf("%w: %q (%d bytes limit)", ErrFileTooLarge, f.Filename, maxSize)
	}
	return content, nil
}

// FormFile returns the first uploaded file for the given field key.
// An optional maxSize caps how many bytes are read for this file;
// exceeding it returns ErrFileTooLarge instead of buffering the rest.
func (c *FastCtx) FormFile(key string, maxSize ...int64) (*FileHeader, error) {
	fh, err := c.FasthttpCtx().MultipartForm()
	if err != nil {
		return nil, err
	}
	files := fh.File[key]
	if len(files) == 0 {
		return nil, nil
	}
	f := files[0]
	fhdr := &FileHeader{
		Filename: f.Filename,
		Size:     int(f.Size),
	}
	var limit int64
	if len(maxSize) > 0 {
		limit = maxSize[0]
	}
	fhdr.content, err = readFilePart(f, limit)
	if err != nil {
		return nil, err
	}
	return fhdr, nil
}

// FormFiles returns all uploaded files for the given field key.
// An optional maxSize caps how many bytes are read per file; a file
// exceeding it aborts with ErrFileTooLarge.
func (c *FastCtx) FormFiles(key string, maxSize ...int64) ([]*FileHeader, error) {
	fh, err := c.FasthttpCtx().MultipartForm()
	if err != nil {
		return nil, err
	}
	files := fh.File[key]
	if len(files) == 0 {
		return nil, nil
	}
	var limit int64
	if len(maxSize) > 0 {
		limit = maxSize[0]
	}
	var result []*FileHeader
	for _, f := range files {
		fhdr := &FileHeader{Filename: f.Filename, Size: int(f.Size)}
		fhdr.content, err = readFilePart(f, limit)
		if err != nil {
			return nil, err
		}
		result = append(result, fhdr)
	}
	return result, nil
}

// SaveFile saves the first uploaded file for the given field key to the given path.
// An optional maxSize caps how many bytes are read for this file.
func (c *FastCtx) SaveFile(key, path string, maxSize ...int64) error {
	fh, err := c.FormFile(key, maxSize...)
	if err != nil {
		return err
	}
	if fh == nil {
		return os.ErrNotExist
	}
	return fh.SaveFile(path)
}

// Set stores a value in the request context (locals).
// Useful for sharing data between middlewares.
func (c *FastCtx) Set(key string, value any) {
	if c.locals == nil {
		c.locals = make(map[string]any)
	}
	c.locals[key] = value
}

// Get retrieves a value from the request context (locals).
func (c *FastCtx) Get(key string) any {
	if c.locals == nil {
		return nil
	}
	return c.locals[key]
}

// Claims returns JWT claims set by JWTMiddleware / BFF auth.
func (c *FastCtx) Claims() (*domain.Claims, error) {
	v := c.Get("claims")
	if v == nil {
		return nil, &domain.UnauthorizedError{Message: "missing claims"}
	}
	claims, ok := v.(*domain.Claims)
	if !ok || claims == nil {
		return nil, &domain.UnauthorizedError{Message: "invalid claims"}
	}
	return claims, nil
}

// UserID returns the authenticated user id from locals (or claims).
func (c *FastCtx) UserID() string {
	if v, ok := c.Get("user_id").(string); ok && v != "" {
		return v
	}
	if claims, err := c.Claims(); err == nil {
		return claims.UserID
	}
	return ""
}

// TenantID returns the current request tenant from locals (set by
// TenantMiddleware / JWTMiddleware) or from JWT claims when present.
func (c *FastCtx) TenantID() string {
	if v, ok := c.Get("tenant_id").(string); ok && v != "" {
		return v
	}
	if claims, err := c.Claims(); err == nil {
		return claims.TenantID
	}
	return ""
}

// SetTenantID stores the tenant id in request locals (used by TenantMiddleware
// and tests). Prefer TenantMiddleware over calling this from handlers.
func (c *FastCtx) SetTenantID(id string) {
	c.Set("tenant_id", id)
}

// UserUUID parses UserID as UUID (v7/v4 string).
func (c *FastCtx) UserUUID() (nitrouuid.UUID, error) {
	id := c.UserID()
	if id == "" {
		return nitrouuid.UUID{}, &domain.UnauthorizedError{Message: "missing user id"}
	}
	u, err := nitrouuid.Parse(id)
	if err != nil {
		return nitrouuid.UUID{}, &domain.BadRequestError{Message: "invalid user uuid"}
	}
	return u, nil
}

// UserInt64 parses UserID as int64.
func (c *FastCtx) UserInt64() (int64, error) {
	id := c.UserID()
	if id == "" {
		return 0, &domain.UnauthorizedError{Message: "missing user id"}
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, &domain.BadRequestError{Message: "invalid user id"}
	}
	return n, nil
}

// Redirect redirects the client to a URL.
// default status: 302 Found.
// Caution: absolute external URLs allow open redirect if they come from user input.
// Prefer SafeRedirect for destinations derived from query/form.
func (c *FastCtx) Redirect(url string, status ...int) error {
	code := fasthttp.StatusFound
	if len(status) > 0 {
		code = status[0]
	}
	c.FasthttpCtx().Redirect(url, code)
	return nil
}

// SafeRedirect redirects only to safe relative paths (start with "/" and
// not "//"), mitigating open redirect. For external hosts, use Redirect with an allowlist.
func (c *FastCtx) SafeRedirect(path string, status ...int) error {
	if !isSafeRedirectPath(path) {
		return domain.NewBadRequest("invalid redirect path")
	}
	return c.Redirect(path, status...)
}

func isSafeRedirectPath(path string) bool {
	if path == "" || path[0] != '/' {
		return false
	}
	if strings.HasPrefix(path, "//") {
		return false
	}
	if strings.ContainsAny(path, "\r\n\\") {
		return false
	}
	return true
}

// RemoteIP returns the peer address without trusting proxy headers.
func (c *FastCtx) RemoteIP() string {
	return c.FasthttpCtx().RemoteIP().String()
}

// ClientIP returns the client IP.
// Convention: forwarded headers are honored only when the direct peer is a
// trusted proxy — by default loopback/private networks (a typical reverse proxy
// or load balancer); override with Config.TrustedProxies or SetTrustedProxies. X-Forwarded-For is read RIGHT to
// LEFT, skipping trusted proxy hops: each proxy appends the address it saw, so
// the left side is attacker-controlled and the first untrusted entry from the
// right is the real client.
func (c *FastCtx) ClientIP() string {
	remote := c.RemoteIP()
	ts := c.trust()
	if !ts.contains(net.ParseIP(remote)) {
		return remote
	}
	if xff := c.Header("X-Forwarded-For"); xff != "" {
		if ip := rightmostUntrustedIP(xff, ts.nets, nil); ip != "" {
			return ip
		}
	}
	if xri := strings.TrimSpace(c.Header("X-Real-IP")); xri != "" && net.ParseIP(xri) != nil {
		return xri
	}
	return remote
}

// Protocol returns the request protocol (http or https).
// X-Forwarded-Proto is honored only from trusted proxies and only for the
// values "http" / "https".
func (c *FastCtx) Protocol() string {
	if c.FasthttpCtx().IsTLS() {
		return "https"
	}
	if c.peerMaySetForwardedHeaders(c.RemoteIP()) {
		switch strings.ToLower(strings.TrimSpace(c.Header("X-Forwarded-Proto"))) {
		case "https":
			return "https"
		case "http":
			return "http"
		}
	}
	return "http"
}

// Hostname returns the request hostname.
func (c *FastCtx) Hostname() string {
	return b2s(c.FasthttpCtx().Host())
}

// BaseURL returns the base URL (protocol + host).
func (c *FastCtx) BaseURL() string {
	return c.Protocol() + "://" + c.Hostname()
}

// IsWebSocket returns true if the request is a WebSocket upgrade.
func (c *FastCtx) IsWebSocket() bool {
	upgrade := b2s(c.FasthttpCtx().Request.Header.Peek("Upgrade"))
	return upgrade == "websocket"
}

// ── Phase 3: Typed Path Params ──────────────────────────────

// Param returns the route parameter. Optional second argument: default value.
//
//	id := c.Param("id")
//	id := c.Param("id", "0")
func (c *FastCtx) Param(name string, defaultVal ...string) string {
	v := c.Ctx.Param(name)
	if v == "" && len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return v
}

// ParamInt returns the route parameter as int with a default.
func (c *FastCtx) ParamInt(name string, defaultVal int) int {
	v := c.Ctx.Param(name)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// ParamInt64 returns the route parameter as int64 with a default.
func (c *FastCtx) ParamInt64(name string, defaultVal int64) int64 {
	v := c.Ctx.Param(name)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return n
}

// Int64Param returns the route parameter as int64, or BadRequest if invalid.
func (c *FastCtx) Int64Param(name string) (int64, error) {
	v := c.Param(name)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, domain.NewBadRequest("invalid " + name)
	}
	return n, nil
}

// ParamFloat returns the route parameter as float64 with a default.
func (c *FastCtx) ParamFloat(name string, defaultVal float64) float64 {
	v := c.Ctx.Param(name)
	if v == "" {
		return defaultVal
	}
	var n float64
	_, err := fmt.Sscanf(v, "%f", &n)
	if err != nil {
		return defaultVal
	}
	return n
}

// ── Phase 3: IntE (error-returning query parser) ─────────────

// IntE returns a query parameter as int with error (no silent default).
// For required parameters where the caller must know missing/invalid.
func (c *FastCtx) IntE(key string) (int, error) {
	v := c.FasthttpCtx().QueryArgs().Peek(key)
	if len(v) == 0 {
		return 0, fmt.Errorf("query parameter %q is required", key)
	}
	n, err := fasthttp.ParseUint(v)
	if err != nil {
		return 0, fmt.Errorf("query parameter %q is not a valid integer: %w", key, err)
	}
	return n, nil
}

// QuerySlice returns all values of a repeated query parameter as []string.
// Supports ?tag=go&tag=api&tag=nitro.
func (c *FastCtx) QuerySlice(key string) []string {
	multi := c.FasthttpCtx().QueryArgs().PeekMulti(key)
	if len(multi) == 0 {
		return nil
	}
	out := make([]string, len(multi))
	for i, v := range multi {
		out[i] = string(v)
	}
	return out
}

func parseQueryBool(v []byte) bool {
	switch len(v) {
	case 1:
		return v[0] == '1' || v[0] == 't' || v[0] == 'T' || v[0] == 'y' || v[0] == 'Y'
	case 3: // yes
		return (v[0]|0x20) == 'y' && (v[1]|0x20) == 'e' && (v[2]|0x20) == 's'
	case 4: // true
		return (v[0]|0x20) == 't' && (v[1]|0x20) == 'r' && (v[2]|0x20) == 'u' && (v[3]|0x20) == 'e'
	case 2: // on
		return (v[0]|0x20) == 'o' && (v[1]|0x20) == 'n'
	}
	return false
}

// ── Phase 3: Aliases ─────────────────────────────────────────

// Context returns the request context.Context (alias for RequestContext).
func (c *FastCtx) Context() context.Context {
	return c.RequestContext()
}

// Deadline implements context.Context — allows passing c as a context.
func (c *FastCtx) Deadline() (time.Time, bool) {
	return c.RequestContext().Deadline()
}

// Done implements context.Context.
func (c *FastCtx) Done() <-chan struct{} {
	return c.RequestContext().Done()
}

// Err implements context.Context.
func (c *FastCtx) Err() error {
	return c.RequestContext().Err()
}

// Value implements context.Context.
func (c *FastCtx) Value(key any) any {
	return c.RequestContext().Value(key)
}

// IP returns the client IP (alias for ClientIP).
func (c *FastCtx) IP() string {
	return c.ClientIP()
}

// ── Phase 3: Predicates ──────────────────────────────────────

// IsJSON returns true if the request Content-Type is application/json.
func (c *FastCtx) IsJSON() bool {
	ct := b2s(c.FasthttpCtx().Request.Header.Peek("Content-Type"))
	return strings.HasPrefix(ct, "application/json")
}

// IsPost returns true if the HTTP method is POST.
func (c *FastCtx) IsPost() bool {
	return c.Method() == "POST"
}

// IsGet returns true if the HTTP method is GET.
func (c *FastCtx) IsGet() bool {
	return c.Method() == "GET"
}

// IsPut returns true if the HTTP method is PUT.
func (c *FastCtx) IsPut() bool {
	return c.Method() == "PUT"
}

// IsDelete returns true if the HTTP method is DELETE.
func (c *FastCtx) IsDelete() bool {
	return c.Method() == "DELETE"
}

// IsPatch returns true if the HTTP method is PATCH.
func (c *FastCtx) IsPatch() bool {
	return c.Method() == "PATCH"
}

// ── Phase 3: Paginated Response ──────────────────────────────

// Paginated returns 200 with the standard pagination structure.
func (c *FastCtx) Paginated(items any, total, page, perPage int) error {
	totalPages := (total + perPage - 1) / perPage
	return c.OK(map[string]any{
		"items":       items,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// ── Response Helpers (v1) ────────────────────────────────

// OK returns 200 with a JSON body.
func (c *FastCtx) OK(v any) error {
	return c.JSON(200, v)
}

// OKOrNil responds 200 with a JSON body if err == nil or err is a not-found error.
// For other errors, returns the original error.
//
//	return c.OKOrNil(nitro.Map{"driver": d}, err)
func (c *FastCtx) OKOrNil(v any, err error) error {
	if err != nil {
		if isNotFoundErr(err) {
			return c.OK(v)
		}
		return err
	}
	return c.OK(v)
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrNotFound) {
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	type notFounder interface{ NotFound() bool }
	var nf notFounder
	return errors.As(err, &nf) && nf.NotFound()
}

// Created returns 201 with a JSON body.
func (c *FastCtx) Created(v any) error {
	return c.JSON(201, v)
}

// Accepted returns 202 with a JSON body.
func (c *FastCtx) Accepted(v any) error {
	return c.JSON(202, v)
}

// NoContent returns 204 with no body.
func (c *FastCtx) NoContent() error {
	c.FasthttpCtx().SetStatusCode(fasthttp.StatusNoContent)
	c.FasthttpCtx().Response.SetBody(nil)
	return nil
}

// Text returns text/plain with status 200 (or custom).
func (c *FastCtx) Text(msg string, status ...int) error {
	code := fasthttp.StatusOK
	if len(status) > 0 {
		code = status[0]
	}
	fctx := c.FasthttpCtx()
	fctx.SetStatusCode(code)
	fctx.Response.Header.SetContentTypeBytes(contentTypeTextUTF8)
	fctx.SetBodyString(msg)
	return nil
}

// Bytes returns raw bytes with content-type octet-stream.
// data is referenced without copy (SetBodyRaw) — do not mutate until the response is sent.
func (c *FastCtx) Bytes(statusCode int, data []byte, contentType ...string) error {
	fctx := c.FasthttpCtx()
	fctx.SetStatusCode(statusCode)
	if len(contentType) > 0 {
		fctx.Response.Header.SetContentType(contentType[0])
	} else {
		fctx.Response.Header.SetContentTypeBytes(contentTypeOctet)
	}
	fctx.Response.SetBodyRaw(data)
	return nil
}

// Write sends pre-serialized JSON (zero json.Marshal).
// data is referenced without copy (SetBodyRaw), like Fiber Send — do not mutate the
// slice until the response is sent (OK for literals / static buffers).
func (c *FastCtx) Write(statusCode int, data []byte) error {
	fctx := c.FasthttpCtx()
	fctx.SetStatusCode(statusCode)
	fctx.Response.Header.SetContentTypeBytes(contentTypeJSON)
	fctx.Response.SetBodyRaw(data)
	return nil
}

// FileOptions holds options for the File method.
type FileOptions struct {
	// Disposition sets Content-Disposition: "attachment" or "inline".
	// Default: "attachment".
	Disposition string
	// ContentType sets the file Content-Type.
	// If empty, detected from the extension or "application/octet-stream".
	ContentType string
}

// File returns a file for download.
// Supports 3 usage forms:
//
//	c.File("photo.jpg", data)                    // automatic detection
//	c.File("photo.jpg", data, "inline")          // custom disposition
//	c.File("photo.jpg", data, FileOptions{...})  // full options
func (c *FastCtx) File(name string, data []byte, opts ...any) error {
	fctx := c.FasthttpCtx()
	fctx.SetStatusCode(fasthttp.StatusOK)

	disp := "attachment"
	ct := detectContentType(name, data)

	for _, opt := range opts {
		switch v := opt.(type) {
		case string:
			disp = v
		case FileOptions:
			if v.Disposition != "" {
				disp = v.Disposition
			}
			if v.ContentType != "" {
				ct = v.ContentType
			}
		}
	}

	fctx.Response.Header.SetContentType(ct)
	fctx.Response.Header.Set("Content-Disposition", contentDisposition(disp, name))
	fctx.SetBody(data)
	return nil
}

// Attachment sends bytes as a download (Content-Disposition: attachment).
// Content-Type: optional argument, otherwise detected from extension / magic bytes.
//
//	return c.Attachment("report.pdf", pdfBytes)
//	return c.Attachment("data.bin", raw, "application/octet-stream")
func (c *FastCtx) Attachment(filename string, data []byte, contentType ...string) error {
	ct := ""
	if len(contentType) > 0 {
		ct = contentType[0]
	}
	if ct == "" {
		ct = detectContentType(filename, data)
	}
	return c.File(filename, data, FileOptions{Disposition: "attachment", ContentType: ct})
}

// CSV builds a CSV (header + rows) and responds as an attachment download.
//
//	return c.CSV("drivers.csv",
//	    []string{"name", "phone"},
//	    [][]string{{"Ada", "1199"}, {"Bob", "2199"}},
//	)
func (c *FastCtx) CSV(filename string, header []string, rows [][]string) error {
	if filename == "" {
		filename = "export.csv"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".csv") {
		filename += ".csv"
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if len(header) > 0 {
		if err := w.Write(header); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return c.Attachment(filename, buf.Bytes(), "text/csv; charset=utf-8")
}

// contentDisposition builds an RFC 6266 header from an untrusted file name:
// control characters, quotes and backslashes are dropped (no header
// injection, no escaped closing quote), path separators are neutralized, and
// non-ASCII names get an ASCII fallback plus filename*=UTF-8”<pct-encoded>.
func contentDisposition(disp, name string) string {
	var ascii strings.Builder
	clean := make([]rune, 0, len(name))
	nonASCII := false
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f || r == '"' || r == '\\':
			continue
		case r == '/':
			r = '_'
		}
		clean = append(clean, r)
		if r > 0x7e {
			nonASCII = true
			ascii.WriteByte('_')
		} else {
			ascii.WriteRune(r)
		}
	}
	fallback := strings.TrimSpace(ascii.String())
	if fallback == "" {
		fallback = "download"
	}
	h := fmt.Sprintf(`%s; filename="%s"`, disp, fallback)
	if nonASCII {
		h += "; filename*=UTF-8''" + url.PathEscape(string(clean))
	}
	return h
}

// detectContentType detects content-type from byte prefix or extension.
func detectContentType(name string, data []byte) string {
	if len(data) > 0 {
		switch {
		case data[0] == 0xFF && len(data) > 2 && data[1] == 0xD8:
			return "image/jpeg"
		case data[0] == 0x89 && len(data) > 3 && string(data[1:4]) == "PNG":
			return "image/png"
		case data[0] == 0x47 && len(data) > 2 && data[1] == 0x49 && data[2] == 0x46:
			return "image/gif"
		case data[0] == 0x25 && len(data) > 3 && data[1] == 0x50 && data[2] == 0x44 && data[3] == 0x46:
			return "application/pdf"
		case data[0] == 0x50 && len(data) > 1 && data[1] == 0x4B:
			return "application/zip"
		}
	}

	ext := strings.ToLower(name)
	switch {
	case strings.HasSuffix(ext, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(ext, ".json"):
		return "application/json"
	case strings.HasSuffix(ext, ".xml"):
		return "application/xml"
	case strings.HasSuffix(ext, ".csv"):
		return "text/csv"
	case strings.HasSuffix(ext, ".txt"):
		return "text/plain"
	case strings.HasSuffix(ext, ".html"):
		return "text/html"
	case strings.HasSuffix(ext, ".css"):
		return "text/css"
	case strings.HasSuffix(ext, ".js"):
		return "application/javascript"
	case strings.HasSuffix(ext, ".png"):
		return "image/png"
	case strings.HasSuffix(ext, ".jpg"), strings.HasSuffix(ext, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(ext, ".gif"):
		return "image/gif"
	case strings.HasSuffix(ext, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(ext, ".webp"):
		return "image/webp"
	case strings.HasSuffix(ext, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(ext, ".webm"):
		return "video/webm"
	case strings.HasSuffix(ext, ".zip"):
		return "application/zip"
	case strings.HasSuffix(ext, ".gz"):
		return "application/gzip"
	}

	return "application/octet-stream"
}

// Stream sends a data stream (SSE, CSV, etc).
func (c *FastCtx) Stream(fn func(io.Writer) error) error {
	fctx := c.FasthttpCtx()
	w := fctx.Response.BodyWriter()
	return fn(w)
}

// ── Error Responses (v1) ──────────────────────────────────

// Error returns a JSON error with a custom status.
func (c *FastCtx) Error(statusCode int, msg string) error {
	return c.JSON(statusCode, map[string]string{"error": msg})
}

// BadRequest returns 400 with an error message.
func (c *FastCtx) BadRequest(msg string) error {
	return c.JSON(fasthttp.StatusBadRequest, map[string]string{"error": msg})
}

// Unauthorized returns 401 with an error message.
func (c *FastCtx) Unauthorized(msg ...string) error {
	m := "unauthorized"
	if len(msg) > 0 {
		m = msg[0]
	}
	return c.JSON(fasthttp.StatusUnauthorized, map[string]string{"error": m})
}

// Forbidden returns 403 with an error message.
func (c *FastCtx) Forbidden(msg ...string) error {
	m := "forbidden"
	if len(msg) > 0 {
		m = msg[0]
	}
	return c.JSON(fasthttp.StatusForbidden, map[string]string{"error": m})
}

// NotFound returns 404 with an error message.
func (c *FastCtx) NotFound(msg string) error {
	return c.JSON(fasthttp.StatusNotFound, map[string]string{"error": msg})
}

// Conflict returns 409 with an error message.
func (c *FastCtx) Conflict(msg string) error {
	return c.JSON(fasthttp.StatusConflict, map[string]string{"error": msg})
}

// UnprocessableEntity returns 422 with an error message.
func (c *FastCtx) UnprocessableEntity(msg ...string) error {
	m := "unprocessable entity"
	if len(msg) > 0 {
		m = msg[0]
	}
	return c.JSON(fasthttp.StatusUnprocessableEntity, map[string]string{"error": m})
}

// InternalError returns 500 with a generic message (does not leak internal details).
func (c *FastCtx) InternalError(err error) error {
	_ = err // caller should log the real error server-side
	return c.JSON(fasthttp.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// ensure interfaces
var _ io.Reader = nil
