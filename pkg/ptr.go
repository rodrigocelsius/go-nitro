package nitro

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// Deref returns *p, or fallback if p is nil.
func Deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// Contains reports whether v is in list.
func Contains[T comparable](list []T, v T) bool {
	for i := range list {
		if list[i] == v {
			return true
		}
	}
	return false
}
