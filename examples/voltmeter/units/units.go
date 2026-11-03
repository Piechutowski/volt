// Package units holds the Go types Voltmeter's schema declares as column
// types with `from` (spec §types): each implements sql.Scanner and
// driver.Valuer, which the generated code asserts.
package units

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Millis is a duration in milliseconds as the profiling sweep reports it,
// stored as REAL (spec §types).
type Millis float64

// String renders the duration with a unit, to a tenth of a millisecond
// below ten seconds and in seconds above.
func (m Millis) String() string {
	if m >= 10000 {
		return strconv.FormatFloat(float64(m)/1000, 'f', 2, 64) + " s"
	}
	return strconv.FormatFloat(float64(m), 'f', 1, 64) + " ms"
}

// Scan implements sql.Scanner over the REAL or INTEGER SQLite returns.
func (m *Millis) Scan(src any) error {
	switch v := src.(type) {
	case float64:
		*m = Millis(v)
	case int64:
		*m = Millis(v)
	default:
		return fmt.Errorf("units.Millis: cannot scan %T", src)
	}
	return nil
}

// Value implements driver.Valuer: the milliseconds as a float.
func (m Millis) Value() (driver.Value, error) { return float64(m), nil }

// Commit is a git commit id, short or full, stored as lower-case hex text
// whatever case it was written in, so one commit is one value (spec §types).
type Commit string

// CommitParse reads s as a commit id: four to forty hex digits, in any
// case, normalized to lower case.
func CommitParse(s string) (Commit, error) {
	low := strings.ToLower(strings.TrimSpace(s))
	if len(low) < 4 || len(low) > 40 {
		return "", fmt.Errorf("commit %q: want 4 to 40 hex digits", s)
	}
	if strings.Trim(low, "0123456789abcdef") != "" {
		return "", fmt.Errorf("commit %q: not hex", s)
	}
	return Commit(low), nil
}

// Short is the first seven digits of the id, the abbreviation git prints.
func (c Commit) Short() string {
	if len(c) > 7 {
		return string(c[:7])
	}
	return string(c)
}

// Scan implements sql.Scanner, normalizing the stored text to lower case.
func (c *Commit) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*c = Commit(strings.ToLower(v))
	case []byte:
		*c = Commit(strings.ToLower(string(v)))
	default:
		return fmt.Errorf("units.Commit: cannot scan %T", src)
	}
	return nil
}

// Value implements driver.Valuer: the id in lower case, so a mixed-case
// id binds as the stored one does.
func (c Commit) Value() (driver.Value, error) { return strings.ToLower(string(c)), nil }
