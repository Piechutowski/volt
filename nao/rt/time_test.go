package rt

import (
	"database/sql/driver"
	"testing"
	"time"
)

// TestTimeScans proves a NOT NULL time.Time reads the driver's value, the
// text SQLite answers for min or max in every layout SQLite or the driver
// writes, and unix seconds or milliseconds as the driver tells them, all
// in UTC, and refuses NULL, text that is no
// date/time and any other type (spec §go-types).
func TestTimeScans(t *testing.T) {
	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	for _, src := range []any{
		want, want.In(time.FixedZone("plus2", 2*3600)), "2026-03-01 12:00:00", "2026-03-01 12:00:00+00:00",
		[]byte("2026-03-01T14:00:00+02:00"), "2026-03-01T12:00:00Z", "2026-03-01 12:00", "2026-03-01T12:00:00.000", int64(1772366400),
	} {
		var got time.Time
		if err := (*Time)(&got).Scan(src); err != nil || !got.Equal(want) || got.Location() != time.UTC {
			t.Errorf("Scan(%v) = %v, %v; want %v in UTC", src, got, err, want)
		}
	}
	ms := want.Add(123 * time.Millisecond)
	var got time.Time
	if err := (*Time)(&got).Scan(ms.UnixMilli()); err != nil || !got.Equal(ms) {
		t.Errorf("Scan(%d) = %v, %v; want unix milliseconds, as the driver reads a column, %v", ms.UnixMilli(), got, err, ms)
	}
	for _, src := range []any{nil, "yesterday", "2026-03-01 12:00:00 +0200", 12, 1.5} {
		var got time.Time
		if err := (*Time)(&got).Scan(src); err == nil {
			t.Errorf("Scan(%v) = %v, want an error", src, got)
		}
	}
}

// TestTimeText proves the stored form is SQLite's datetime text in UTC,
// fractional seconds only when not zero and trimmed, and that a year
// SQLite cannot store is an error (spec §go-types).
func TestTimeText(t *testing.T) {
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 11, 3, 5, 18, 49, 0, time.UTC), "2026-11-03 05:18:49"},
		{time.Date(2026, 11, 3, 7, 18, 49, 500_000_000, time.FixedZone("plus2", 2*3600)), "2026-11-03 05:18:49.5"},
		{time.Date(2026, 11, 3, 5, 18, 49, 123_456_789, time.UTC), "2026-11-03 05:18:49.123456789"},
		{time.Date(2026, 11, 3, 5, 18, 49, 120_000_000, time.UTC), "2026-11-03 05:18:49.12"},
		{time.Time{}, "0001-01-01 00:00:00"},
		{time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), "9999-12-31 23:59:59"},
	} {
		if got, err := TimeText(c.at); err != nil || got != c.want {
			t.Errorf("TimeText(%v) = %q, %v; want %q", c.at, got, err, c.want)
		}
	}
	for _, at := range []time.Time{time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC)} {
		if got, err := TimeText(at); err == nil {
			t.Errorf("TimeText(%v) = %q, want an error", at, got)
		}
	}
}

type stamp time.Time

func (s stamp) Value() (driver.Value, error) { return time.Time(s), nil }

// TestArgsBoundCanonical proves every time.Time argument, every valid
// rt.Null[time.Time] and every driver.Valuer whose value is a time.Time
// binds as its TimeText, that arguments with nothing to change are handed
// on as they are, and that an unstorable year is an error (spec §go-types).
func TestArgsBoundCanonical(t *testing.T) {
	at := time.Date(2026, 3, 1, 13, 0, 0, 0, time.FixedZone("plus2", 2*3600))
	const text = "2026-03-01 11:00:00"
	var nilStamp *stamp
	args := []any{1, at, "x", Some(at), stamp(at), nilStamp, Bytes(""), Null[time.Time]{}}
	bound, err := ArgsBound(args)
	if err != nil {
		t.Fatal(err)
	}
	if bound[1] != text || bound[4] != text {
		t.Errorf("bound %v and %v, want %q", bound[1], bound[4], text)
	}
	if bound[3] != any(Some(at)) || bound[5] != any(nilStamp) || bound[7] != any(Null[time.Time]{}) {
		t.Errorf("bound %v, %v, %v; a Valuer whose value is no time.Time binds as it is", bound[3], bound[5], bound[7])
	}
	if _, ok := args[1].(time.Time); !ok {
		t.Error("ArgsBound changed its input")
	}
	plain := []any{1, "2026-03-01 11:00:00", Bytes("\x01")}
	if got, err := ArgsBound(plain); err != nil || &got[0] != &plain[0] {
		t.Error("ArgsBound copied arguments it did not change")
	}
	if _, err := ArgsBound([]any{time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}); err == nil {
		t.Error("a year past 9999 bound without an error")
	}
	v, err := Some(at).Value()
	if err != nil || v != text {
		t.Errorf("Some(at).Value() = %v, %v, want %q", v, err, text)
	}
	if got := JSONList([]time.Time{at, at.Add(500 * time.Millisecond)}).String(); got != `["2026-03-01 11:00:00","2026-03-01 11:00:00.5"]` {
		t.Errorf("JSONList(times) = %s", got)
	}
	if got := JSONList([]stamp{stamp(at)}).String(); got != `["2026-03-01 11:00:00"]` {
		t.Errorf("JSONList(stamps) = %s", got)
	}
}
