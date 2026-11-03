package rt

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// TimeLayout is the one stored form of a time.Time (spec §go-types):
// SQLite's own datetime text, the form CURRENT_TIMESTAMP and datetime()
// write, formatted in UTC, with the fractional seconds only when they
// are not zero and without their trailing zeros.
const TimeLayout = "2006-01-02 15:04:05.999999999"

// TimeText is t as Volt stores and binds it (spec §go-types): TimeLayout
// in UTC, so SQLite's text order is the instants' order. A year outside
// 0000 to 9999, where SQLite's date functions and that order end, is an
// error.
func TimeText(t time.Time) (string, error) {
	u := t.UTC()
	if y := u.Year(); y < 0 || y > 9999 {
		return "", fmt.Errorf("rt: time %s is outside the years 0000 to 9999 SQLite stores (spec §go-types)", t)
	}
	return u.Format(TimeLayout), nil
}

// Time is a time.Time as generated code scans a NOT NULL date/time
// item: the driver's own value for a declared column, or the text or
// integer SQLite answers for min or max of one, which carries no
// declared type for the driver to convert by (spec §selects-over-groups).
// A value reads in UTC (spec §go-types).
type Time time.Time

// Scan reads a time.Time, text in a layout SQLite or a driver writes,
// or integer unix seconds, always in UTC; NULL, any other text and any
// other type are errors.
func (t *Time) Scan(src any) error {
	var text string
	switch v := src.(type) {
	case time.Time:
		*t = Time(v.UTC())
		return nil
	case int64:
		*t = Time(time.Unix(v, 0).UTC())
		return nil
	case string:
		text = v
	case []byte:
		text = string(v)
	case nil:
		return fmt.Errorf("rt: converting NULL to time.Time is unsupported")
	default:
		return fmt.Errorf("rt: converting %T to time.Time is unsupported", src)
	}
	parsed, err := timeOfText(text)
	if err != nil {
		return err
	}
	*t = Time(parsed)
	return nil
}

func timeOfText(text string) (time.Time, error) {
	layouts := [...]string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	s := strings.TrimSuffix(text, "Z")
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("rt: %q is no date/time SQLite writes", text)
}

// timeBound is v as Volt binds it: a time.Time, or a driver.Valuer whose
// value is one, as its TimeText; ok is false for any other value, which
// binds as it is.
func timeBound(v any) (string, bool, error) {
	if t, ok := v.(time.Time); ok {
		s, err := TimeText(t)
		return s, true, err
	}
	dv, ok := v.(driver.Valuer)
	if !ok {
		return "", false, nil
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return "", false, nil
	}
	val, err := dv.Value()
	if err != nil {
		return "", false, nil
	}
	t, ok := val.(time.Time)
	if !ok {
		return "", false, nil
	}
	s, err := TimeText(t)
	return s, true, err
}

// ArgsBound is a statement's arguments as an executor binds them: every
// time.Time, and every driver.Valuer whose value is one, as its TimeText,
// so SQLite's min, max, where, order and date functions read the form
// CURRENT_TIMESTAMP writes (spec §go-types). It copies only when an
// argument changes, and a time outside TimeText's years is an error.
func ArgsBound(args []any) ([]any, error) {
	var out []any
	for i, a := range args {
		s, ok, err := timeBound(a)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if out == nil {
			out = append([]any(nil), args...)
		}
		out[i] = s
	}
	if out == nil {
		return args, nil
	}
	return out, nil
}
