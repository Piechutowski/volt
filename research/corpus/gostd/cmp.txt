package cmp // import "cmp"

Package cmp provides types and functions related to comparing ordered values.

FUNCTIONS

func Compare[T Ordered](x, y T) int
    Compare returns

        -1 if x is less than y,
         0 if x equals y,
        +1 if x is greater than y.

    For floating-point types, a NaN is considered less than any non-NaN,
    a NaN is considered equal to a NaN, and -0.0 is equal to 0.0.

func Less[T Ordered](x, y T) bool
    Less reports whether x is less than y. For floating-point types, a NaN is
    considered less than any non-NaN, and -0.0 is not less than (is equal to)
    0.0.

func Or[T comparable](vals ...T) T
    Or returns the first of its arguments that is not equal to the zero value.
    If no argument is non-zero, it returns the zero value.


TYPES

type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~string
}
    Ordered is a constraint that permits any ordered type: any type that
    supports the operators < <= >= >. If future releases of Go add new ordered
    types, this constraint will be modified to include them.

    Note that floating-point types may contain NaN ("not-a-number") values. An
    operator such as == or < will always report false when comparing a NaN value
    with any other value, NaN or not. See the Compare function for a consistent
    way to compare NaN values.

