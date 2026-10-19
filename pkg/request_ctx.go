package nitro

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// RequestFrom returns the HTTP request carried by a Resource hook context
// (BeforeWrite, BeforeCreate, AfterCreate, OnMutate, …), giving hooks the
// tenant, authenticated user, IP and headers:
//
//	OnMutate: func(ctx context.Context, action string, e any) error {
//	    if c, ok := nitro.RequestFrom(ctx); ok {
//	        audit.Record(ctx, c.TenantID(), c.UserID(), action, e)
//	    }
//	    return nil
//	},
//
// Only valid while the request is being handled.
func RequestFrom(ctx context.Context) (*Ctx, bool) {
	if c, ok := ctx.(*Ctx); ok {
		return c, true
	}
	dc, ok := domain.RequestFrom(ctx)
	if !ok {
		return nil, false
	}
	fc, ok := dc.AdapterData.(*Ctx)
	return fc, ok && fc != nil
}
