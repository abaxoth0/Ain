package common

/*
Ternary operator:
Returns "a" if "cond" is true otherwise returns "b".

Usage-rules:

1 - Do not nest this function: util.Ternary(cond1, util.Ternary(cond2, a, b), c) [ WRONG ]

2 - Use it only with primitives: util.Ternary(newState == user.DeletedState, "IS NOT", "IS") [CORRECT]

3 - Do not place function calls inside of it: util.Ternary(cond, calcA(), calcB()) [WRONG]

	Exceptions:
		- private fields/vars getters: util.Ternary(cond, someStruct.GetA(), anotherStruct.GetB()) [CORRECT]
*/
func Ternary[T any](cond bool, a T, b T) T {
	if cond { return a }; return b
}
