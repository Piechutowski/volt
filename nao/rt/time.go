package rt

import (
	"fmt"
	"strings"
	"time"
)

// Time is a time.Time as generated code scans a NOT NULL date/time
// item: the driver's own value for a declared column, or the text SQLite
// answers for min or max of one, which carries no declared type for the
// driver to convert by (spec §selects-over-groups). Every time.Time binds
// in UTC, so the text SQLite compares orders as the instants do (spec
// §go-types).
type Time time.Time

// Scan reads a time.Time, or text in a layout a driver or SQLite writes;
// NULL and any other text are errors.
func (t *Time) Scan(src any) error {
	var text string
	switch v := src.(type) {
	case time.Time:
		*t = Time(v)
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
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("rt: %q is no date/time SQLite writes", text)
}

// ArgsBound is a statement's arguments as an executor binds them: every
// time.Time in UTC, so the text the driver writes for one compares, in
// SQLite's min, max, where and order, as the instants do (spec §go-types).
// It copies only when an argument changes.
func ArgsBound(args []any) []any {
	var out []any
	for i, a := range args {
		t, ok := a.(time.Time)
		if !ok || t.Location() == time.UTC {
			continue
		}
		if out == nil {
			out = append([]any(nil), args...)
		}
		out[i] = t.UTC()
	}
	if out == nil {
		return args
	}
	return out
}
