package nitro

// Number is any built-in numeric type usable with SumBy.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

// SumBy calls fn for each item — by pointer, so fn may also mutate the item
// (e.g. to compute a per-item field like Subtotal) — and returns the sum of
// fn's results. Typical use inside a RecalcFunc, replacing a manual for-loop:
//
//	sale.Total = nitro.SumBy(items, func(item *SaleItem) float64 {
//	    item.Subtotal = item.Quantity * item.UnitPrice
//	    return item.Subtotal
//	})
func SumBy[T any, N Number](items []T, fn func(item *T) N) N {
	var total N
	for i := range items {
		total += fn(&items[i])
	}
	return total
}

// CountBy returns how many items satisfy predicate. Handy alongside SumBy
// inside a RecalcFunc, e.g. counting only non-voided items:
//
//	sale.ActiveItems = nitro.CountBy(items, func(item SaleItem) bool { return !item.Voided })
func CountBy[T any](items []T, predicate func(item T) bool) int {
	n := 0
	for _, it := range items {
		if predicate(it) {
			n++
		}
	}
	return n
}
