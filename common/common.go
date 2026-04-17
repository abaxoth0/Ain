package common

// Returns "a" if "cond" is true, otherwise returns "b".
//
// Warning: Both "a" and "b" are evaluated regardless of the condition.
// Use with caution when the expressions have side effects.
func Ternary[T any](cond bool, a T, b T) T {
	if cond {
		return a
	}
	return b
}

// Returns slice of values from specified map
func MapValues[K comparable, V any](m map[K]V) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

// Returns slice of keys from specified map
func MapKeys[K comparable, V any](m map[K]V) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}
