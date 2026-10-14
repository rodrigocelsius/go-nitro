package pgx

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// softColumnOf returns "deleted_at" if T (or an embed) has soft delete.
func softColumnOf[T any]() string {
	var zero T
	return domain.SoftDeleteColumn(reflect.TypeOf(zero))
}
