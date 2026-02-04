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
