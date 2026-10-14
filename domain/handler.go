package domain

// Handler is the signature of a Nitro route handler.
// It is the most referenced type in the framework — it defines the
// contract between the router, middlewares, and application code.
type Handler func(c *Ctx) error

// MaxParams is the maximum number of route parameters supported
// per request without allocation. Routes with more parameters still
// work, but use an allocating fallback (rare in practice).
const MaxParams = 8
