package volt

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// QueryValue is the set of Go types a query-string parameter can carry: the
// scalar column types bool, int, float64, string and time.Time (§columns),
// the only types a query route binds from the query string (§query-routes).
type QueryValue interface {
	string | bool | int | float64 | time.Time
}

// QueryParam binds one scalar parameter from the query string by name, as a
// generated query route does (§query-routes). A missing or unparseable value is
// a 400 naming the parameter.
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

// QueryParams binds a list parameter (§predicates): every value of a repeated
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
	return fmt.Sprintf("%T", dst)[1:]
}

// FormatQuery renders a query-string value the way QueryParam parses it
// (§generated-client).
func FormatQuery[T QueryValue](v T) string {
	switch v := any(v).(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}
