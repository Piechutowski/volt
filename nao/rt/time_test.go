package rt

import (
	"testing"
	"time"
)

// TestTimeScans proves a NOT NULL time.Time reads the driver's value and
// the text SQLite answers for min or max, and refuses NULL and text that
// is no date/time (spec §go-types).
func TestTimeScans(t *testing.T) {
	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	for _, src := range []any{want, "2026-03-01 12:00:00+00:00", []byte("2026-03-01T14:00:00+02:00"), "2026-03-01 12:00"} {
		var got time.Time
		if err := (*Time)(&got).Scan(src); err != nil || !got.Equal(want) {
			t.Errorf("Scan(%v) = %v, %v", src, got, err)
		}
	}
	for _, src := range []any{nil, "yesterday", 12} {
		var got time.Time
		if err := (*Time)(&got).Scan(src); err == nil {
			t.Errorf("Scan(%v) = %v, want an error", src, got)
		}
	}
}

// TestArgsBoundInUTC proves every time.Time argument and every valid
// rt.Null[time.Time] binds in UTC, and that arguments with nothing to
// change are handed on as they are (spec §go-types).
func TestArgsBoundInUTC(t *testing.T) {
	at := time.Date(2026, 3, 1, 13, 0, 0, 0, time.FixedZone("plus2", 2*3600))
	args := []any{1, at, "x"}
	bound := ArgsBound(args)
	if got := bound[1].(time.Time); got.Location() != time.UTC || !got.Equal(at) {
		t.Errorf("bound %v, want %v in UTC", got, at)
	}
	if args[1].(time.Time).Location() == time.UTC {
		t.Error("ArgsBound changed its input")
	}
	plain := []any{1, at.UTC()}
	if got := ArgsBound(plain); &got[0] != &plain[0] {
		t.Error("ArgsBound copied arguments it did not change")
	}
	v, err := Some(at).Value()
	if got, ok := v.(time.Time); err != nil || !ok || got.Location() != time.UTC || !got.Equal(at) {
		t.Errorf("Some(at).Value() = %v, %v, want the instant in UTC", v, err)
	}
}
