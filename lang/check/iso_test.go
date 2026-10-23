package check

import (
	"strings"
	"testing"
	"time"
)

var isoDriverLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// TestISOTimeAgreesWithTheDriverLayouts proves a time.Time record value
// is accepted exactly when one of the layouts the sqlite3 driver reads a
// TIMESTAMP column with parses it, a trailing Z dropped (spec
// §records-sample-data).
func TestISOTimeAgreesWithTheDriverLayouts(t *testing.T) {
	for _, s := range []string{
		"2024-01-15", "2024-01-15 10:30:00", "2024-01-15T10:30:00", "2024-01-15T10:30:00Z",
		"2024-01-15 10:30", "2024-01-15T10:30", "2024-01-15 10:30:00.5", "2024-01-15 10:30:00.123456789",
		"2024-01-15 10:30:00+02:00", "2024-01-15T10:30:00.25-05:30", "2024-01-15 9:30", "2024-02-29",
		"2023-02-29", "2024-13-01", "2024-00-10", "2024-04-31", "2024-01-15 24:00", "2024-01-15 10:60",
		"2024-01-15 10:30:61", "2024-1-15", "24-01-15", "2024/01/15", "not a date at all", "",
		"2024-01-15 10", "2024-01-15 10:30:", "2024-01-15 10:30:00.", "2024-01-15 10:30+02:00",
		"2024-01-15 10:30:00 ", "2024-01-15x", "2024-01-15 10:30:00+2:00",
	} {
		want := false
		for _, layout := range isoDriverLayouts {
			if _, err := time.Parse(layout, strings.TrimSuffix(s, "Z")); err == nil {
				want = true
			}
		}
		if got := isoTimeOK(s); got != want {
			t.Errorf("isoTimeOK(%q) = %v, the driver's layouts say %v", s, got, want)
		}
	}
}
