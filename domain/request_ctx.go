package domain

import "context"

type requestKey struct{}

// WithRequest returns ctx carrying the HTTP request. Resource handlers attach
// it so lifecycle hooks (BeforeCreate, OnMutate, …), which only receive a
// context.Context, can read the tenant, authenticated user, IP or headers.
func WithRequest(ctx context.Context, c *Ctx) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestKey{}, c)
}

// RequestFrom returns the request attached by WithRequest. The request is
// only valid while its handler runs: do not keep it in goroutines.
func RequestFrom(ctx context.Context) (*Ctx, bool) {
	if ctx == nil {
		return nil, false
	}
	c, ok := ctx.Value(requestKey{}).(*Ctx)
	return c, ok && c != nil
}
