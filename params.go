package volt

import "strconv"

// ParseInt parses a decimal int path parameter.
func ParseInt(s string) (int, bool) {
	v, err := strconv.ParseInt(s, 10, 0)
	return int(v), err == nil
}
