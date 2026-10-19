package usecases

import (
	"context"
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// persistUpdate writes a PATCH. When the existing row was loaded (merge mode)
// and the repository supports partial updates, only the fields the request
// actually changed are written: a full-row UPDATE would overwrite columns
// changed concurrently by another request or service with the stale values
// read before the merge (lost update). With nothing changed, nothing is
// written. Replace mode (PUT semantics) keeps the full update.
func persistUpdate(ctx context.Context, repo domain.ResourceRepository, existing, entity any, merge bool) error {
	pu, ok := repo.(domain.ResourcePartialUpdater)
	if !ok || !merge || existing == nil {
		return repo.Update(ctx, entity)
	}
	changed := changedFieldNames(existing, entity)
	if len(changed) == 0 {
		return nil
	}
	return pu.UpdateFields(ctx, entity, changed)
}

// changedFieldNames lists exported fields (promoted through anonymous embeds)
// whose values differ between before and after. System fields managed by the
// repository (ID, CreatedAt, UpdatedAt) are excluded.
func changedFieldNames(before, after any) []string {
	bv := reflect.Indirect(reflect.ValueOf(before))
	av := reflect.Indirect(reflect.ValueOf(after))
	if !bv.IsValid() || !av.IsValid() || bv.Type() != av.Type() || bv.Kind() != reflect.Struct {
		return nil
	}
	var out []string
	var walk func(b, a reflect.Value)
	walk = func(b, a reflect.Value) {
		t := b.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				walk(b.Field(i), a.Field(i))
				continue
			}
			if !f.IsExported() {
				continue
			}
			switch f.Name {
			case "ID", "CreatedAt", "UpdatedAt":
				continue
			}
			if !reflect.DeepEqual(b.Field(i).Interface(), a.Field(i).Interface()) {
				out = append(out, f.Name)
			}
		}
	}
	walk(bv, av)
	return out
}
