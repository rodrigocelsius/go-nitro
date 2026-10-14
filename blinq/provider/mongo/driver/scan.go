package driver

import (
	"fmt"
	"reflect"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

func scanMaps(dest any, rows []map[string]any) error {
	if dest == nil {
		return fmt.Errorf("blinq/mongo/driver: ScanSlice dest nil")
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("blinq/mongo/driver: ScanSlice expects *[]T")
	}
	slice := rv.Elem()
	et := slice.Type().Elem()
	if et.Kind() == reflect.Map {
		out := reflect.MakeSlice(slice.Type(), len(rows), len(rows))
		for i, row := range rows {
			out.Index(i).Set(reflect.ValueOf(row))
		}
		slice.Set(out)
		return nil
	}
	out := reflect.MakeSlice(slice.Type(), 0, len(rows))
	for _, row := range rows {
		elem := reflect.New(et)
		if err := scan.MapToStruct(row, elem.Interface()); err != nil {
			return err
		}
		out = reflect.Append(out, elem.Elem())
	}
	slice.Set(out)
	return nil
}
