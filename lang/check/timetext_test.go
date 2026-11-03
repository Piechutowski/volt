package check

import (
	"testing"
	"time"
)

// TestTimeTextCanonicalIsTheStoredForm proves a time.Time default or
// record value is accepted exactly when it is the text the runtime stores
// for the instant it names: SQLite's datetime text in UTC, fractional
// seconds only when not zero and trimmed (spec §go-types,
// §records-sample-data).
func TestTimeTextCanonicalIsTheStoredForm(t *testing.T) {
	for _, s := range []string{
		"2024-01-15 10:30:00", "2024-01-15 10:30:00.5", "2024-01-15 10:30:00.123456789", "0001-01-01 00:00:00",
		"2024-02-29 23:59:59", "2024-01-15", "2024-01-15T10:30:00", "2024-01-15T10:30:00Z", "2024-01-15 10:30",
		"2024-01-15 10:30:00.500", "2024-01-15 10:30:00.0", "2024-01-15 10:30:00+00:00", "2024-01-15 10:30:00+02:00",
		"2024-01-15 9:30:00", "2023-02-29 00:00:00", "2024-13-01 00:00:00", "2024-01-15 24:00:00", "not a date at all", "",
	} {
		want := false
		if at, err := time.Parse("2006-01-02 15:04:05.999999999", s); err == nil && at.Format("2006-01-02 15:04:05.999999999") == s {
			want = true
		}
		canon, ok := timeTextCanonical(s)
		if got := ok && canon == s; got != want {
			t.Errorf("timeTextCanonical(%q) = %q, %v; canonical %v", s, canon, ok, want)
		}
	}
	for s, want := range map[string]string{
		"2024-01-15":                "2024-01-15 00:00:00",
		"2024-01-15T10:30:00Z":      "2024-01-15 10:30:00",
		"2024-01-15 12:30:00+02:00": "2024-01-15 10:30:00",
		"2024-01-15 10:30:00.500":   "2024-01-15 10:30:00.5",
	} {
		if canon, ok := timeTextCanonical(s); !ok || canon != want {
			t.Errorf("timeTextCanonical(%q) = %q, %v; want %q", s, canon, ok, want)
		}
	}
}

// TestSQLStringOf proves only an expression literal that is one SQL
// string literal is read as time text (spec §default-values).
func TestSQLStringOf(t *testing.T) {
	for expr, want := range map[string]string{"'2024-01-15 10:30:00'": "2024-01-15 10:30:00", "'it''s'": "it's", "''": ""} {
		if got, ok := sqlStringOf(expr); !ok || got != want {
			t.Errorf("sqlStringOf(%s) = %q, %v; want %q", expr, got, ok, want)
		}
	}
	for _, expr := range []string{"CURRENT_TIMESTAMP", "datetime('now')", "'a' || 'b'", "'", "'a'b'"} {
		if got, ok := sqlStringOf(expr); ok {
			t.Errorf("sqlStringOf(%s) = %q, want no string", expr, got)
		}
	}
}
