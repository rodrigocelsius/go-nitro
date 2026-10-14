package sql

import (
	"fmt"
	"reflect"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

func scanMapsToDest(rows []map[string]any, dest any) error {
	if dest == nil {
		return fmt.Errorf("blinq/sql: ScanSlice dest is nil")
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("blinq/sql: ScanSlice dest must be non-nil pointer to slice")
	}
	slice := rv.Elem()
	if slice.Kind() != reflect.Slice {
		return fmt.Errorf("blinq/sql: ScanSlice dest must be *[]T")
	}
	et := slice.Type().Elem()

	// *[]map[string]any
	if et.Kind() == reflect.Map && et.Key().Kind() == reflect.String && et.Elem().Kind() == reflect.Interface {
		out := reflect.MakeSlice(slice.Type(), len(rows), len(rows))
		for i, row := range rows {
			out.Index(i).Set(reflect.ValueOf(row))
		}
		slice.Set(out)
		return nil
	}

	out := reflect.MakeSlice(slice.Type(), 0, len(rows))
	for _, row := range rows {
		elemPtr := reflect.New(et)
		if err := scan.MapToStruct(row, elemPtr.Interface()); err != nil {
			return err
		}
		out = reflect.Append(out, elemPtr.Elem())
	}
	slice.Set(out)
	return nil
}
