package volt

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// QueryValue is the set of Go types a query-string parameter can carry:
// the scalar types the model generator emits for columns (Appendix A),
// which are the types a select parameter can have (§V10.3).
type QueryValue interface {
	string | bool |
		int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64 |
		float32 | float64 |
		time.Time
}

// QueryParam binds one scalar parameter from the query string by name, as a
// generated query route does (§V4.8). A missing or unparseable value is
// a 400 naming the parameter, because a query route's parameters are
// the contract, not options.
func QueryParam[T QueryValue](r *Request, name string) (T, error) {
	var v T
	vals, ok := r.URL.Query()[name]
	if !ok || len(vals) == 0 {
		return v, Error(http.StatusBadRequest, "missing query parameter "+name)
	}
	if err := parseQuery(vals[0], &v); err != nil {
		return v, Error(http.StatusBadRequest, fmt.Sprintf("query parameter %s: %v", name, err))
	}
	return v, nil
}

// QueryParams binds a list parameter (§V10.3): every value of a repeated
// query key, in order. An absent key is an empty list, never an error;
// an element that does not parse is a 400 naming the parameter.
func QueryParams[T QueryValue](r *Request, name string) ([]T, error) {
	vals := r.URL.Query()[name]
	out := make([]T, 0, len(vals))
	for _, s := range vals {
		var v T
		if err := parseQuery(s, &v); err != nil {
			return nil, Error(http.StatusBadRequest, fmt.Sprintf("query parameter %s: %v", name, err))
		}
		out = append(out, v)
	}
	return out, nil
}

// parseQuery parses one query-string value into the typed destination.
// Times are RFC 3339, which is also how they render in JSON.
func parseQuery(s string, dst any) error {
	var err error
	switch d := dst.(type) {
	case *string:
		*d = s
	case *bool:
		*d, err = strconv.ParseBool(s)
	case *int:
		var v int64
		v, err = strconv.ParseInt(s, 10, 0)
		*d = int(v)
	case *int8:
		var v int64
		v, err = strconv.ParseInt(s, 10, 8)
		*d = int8(v)
	case *int16:
		var v int64
		v, err = strconv.ParseInt(s, 10, 16)
		*d = int16(v)
	case *int32:
		var v int64
		v, err = strconv.ParseInt(s, 10, 32)
		*d = int32(v)
	case *int64:
		*d, err = strconv.ParseInt(s, 10, 64)
	case *uint:
		var v uint64
		v, err = strconv.ParseUint(s, 10, 0)
		*d = uint(v)
	case *uint8:
		var v uint64
		v, err = strconv.ParseUint(s, 10, 8)
		*d = uint8(v)
	case *uint16:
		var v uint64
		v, err = strconv.ParseUint(s, 10, 16)
		*d = uint16(v)
	case *uint32:
		var v uint64
		v, err = strconv.ParseUint(s, 10, 32)
		*d = uint32(v)
	case *uint64:
		*d, err = strconv.ParseUint(s, 10, 64)
	case *float32:
		var v float64
		v, err = strconv.ParseFloat(s, 32)
		*d = float32(v)
	case *float64:
		*d, err = strconv.ParseFloat(s, 64)
	case *time.Time:
		*d, err = time.Parse(time.RFC3339, s)
	default:
		return fmt.Errorf("unsupported parameter type %T", dst)
	}
	if err != nil {
		return fmt.Errorf("%q is not a valid %s", s, typeName(dst))
	}
	return nil
}

func typeName(dst any) string {
	return fmt.Sprintf("%T", dst)[1:] // strip the pointer star
}

// FormatQuery renders a query-string value the way QueryParam parses it:
// the inverse a generated client uses to build URLs (§V4.10).
func FormatQuery[T QueryValue](v T) string {
	switch v := any(v).(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return fmt.Sprint(v) // the integer families
	}
}
