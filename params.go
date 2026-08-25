package volt

import "strconv"

// Typed path-parameter parsers used by generated shims. A false report
// means the segment does not spell a value of the type; the shim
// returns ErrNotFound (R6).

// ParseInt parses a decimal int path parameter.
func ParseInt(s string) (int, bool) {
	v, err := strconv.ParseInt(s, 10, 0)
	return int(v), err == nil
}

// ParseInt32 parses a decimal int32 path parameter.
func ParseInt32(s string) (int32, bool) {
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err == nil
}

// ParseInt64 parses a decimal int64 path parameter.
func ParseInt64(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}
