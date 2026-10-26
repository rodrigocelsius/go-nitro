package nitro

import (
	"context"
	"fmt"
	"reflect"
)

// getter is the minimal Store surface needed by FillStringFrom.
type getter[T any] interface {
	Get(ctx context.Context, id any) (*T, error)
}

// FillStringFrom loads related by id and copies a string field into dest.
// No-op when id is nil/empty. Used in BeforeWrite for denormalized names:
//
//	nitro.FillStringFrom(ctx, stores.Artists, album.ArtistID, &album.ArtistName, func(a *Artist) string { return a.Name })
func FillStringFrom[T any](
	ctx context.Context,
	store getter[T],
	id any,
	dest *string,
	pick func(*T) string,
) error {
	if dest == nil || pick == nil || store == nil {
		return nil
	}
	if id == nil || isEmptyLookupID(id) {
		return nil
	}
	row, err := store.Get(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	*dest = pick(row)
	return nil
}

func isEmptyLookupID(id any) bool {
	// A nil pointer (e.g. an optional FK `*nitro.UUID`) is empty; checked
	// first because calling String() through it would panic.
	if rv := reflect.ValueOf(id); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return true
	}
	switch v := id.(type) {
	case string:
		return v == ""
	case *string:
		return v == nil || *v == ""
	case fmt.Stringer:
		return v.String() == "" || v.String() == "00000000-0000-0000-0000-000000000000"
	default:
		if v == nil {
			return true
		}
		s := fmt.Sprint(v)
		return s == "" || s == "<nil>" || s == "0"
	}
}
