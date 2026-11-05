package check

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

// TestTimeExprStored proves which expression literals a time.Time default
// or record value may be: those SQLite evaluates to the stored form, and
// none that writes another form, another zone or another type (spec
// §default-values). TestTimeExprStoredOnSQLite runs them on SQLite.
func TestTimeExprStored(t *testing.T) {
	for _, e := range timeExprsStored {
		if !timeExprStored(sqlUnparen(e)) {
			t.Errorf("%s is refused, but SQLite writes the stored form", e)
		}
	}
	for _, e := range timeExprsNotStored {
		if timeExprStored(sqlUnparen(e)) {
			t.Errorf("%s is accepted, but SQLite does not write the stored form", e)
		}
	}
}

var timeExprsStored = []string{
	"CURRENT_TIMESTAMP", "current_timestamp", "(CURRENT_TIMESTAMP)", "datetime('now')", "DATETIME('now', '+1 day')",
	"datetime ('now', 'start of day')", "datetime(1700000000, 'unixepoch')", "datetime('2024-01-15T12:30:00+02:00')",
	"((datetime('now', '-' || 7 || ' days')))", "datetime('2026-03-01 12:00:00.120')",
}

var timeExprsNotStored = []string{
	"datetime('now', 'subsec')", "datetime('now', ' SUBSEC ')", "datetime('2026-03-01 12:00:00', 'subsec')",
	"strftime('%Y-%m-%d %H:%M:%f', 'now')", "julianday('now')", "unixepoch()",
	"datetime('now', 'localtime')", "datetime('2024-01-15 10:00:00', 'utc')", "datetime('now') || ''", "date('now')",
	"CURRENT_DATE", "time('now')", "datetime", "datetimes('now')", "(datetime('now')) || (datetime('now'))",
}

// TestTimeExprStoredOnSQLite proves on SQLite that every accepted
// expression writes the stored form, and that the refused ones whose
// result does not hang on the clock or the server's zone write another
// form or another type (spec §default-values).
func TestTimeExprStoredOnSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, e := range timeExprsStored {
		var v any
		if err := db.QueryRow("SELECT " + e).Scan(&v); err != nil {
			t.Fatalf("%s: %v", e, err)
		}
		s, ok := v.(string)
		if canon, isTime := timeTextCanonical(s); !ok || !isTime || canon != s {
			t.Errorf("%s writes %#v, not the stored form", e, v)
		}
	}
	for _, e := range []string{
		"datetime('2026-03-01 12:00:00.120', 'subsec')", "datetime('2026-03-01 12:00:00', 'subsec')",
		"strftime('%Y-%m-%d %H:%M:%f', '2026-03-01 12:00:00')", "julianday('2026-03-01')", "unixepoch('2026-03-01')",
		"date('2026-03-01')",
	} {
		var v any
		if err := db.QueryRow("SELECT " + e).Scan(&v); err != nil {
			t.Fatalf("%s: %v", e, err)
		}
		if s, ok := v.(string); ok {
			if canon, isTime := timeTextCanonical(s); isTime && canon == s {
				t.Errorf("%s writes the stored form %q, but the checker refuses it", e, s)
			}
		}
	}
}
