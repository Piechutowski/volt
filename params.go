package volt

import "strconv"

// ParseInt parses a decimal int path parameter.
func ParseInt(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}
