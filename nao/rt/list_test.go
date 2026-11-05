package rt

import (
	"database/sql/driver"
	"math"
	"testing"
	"time"
)

func TestJSONList(t *testing.T) {
	if got := JSONList[string](nil).String(); got != "[]" {
		t.Errorf("nil = %q, want []", got)
	}
	if got := JSONList([]string{"a", `b"c`}).String(); got != `["a","b\"c"]` {
		t.Errorf("strings = %q", got)
	}
	if got := JSONList([]int32{1, -2}).String(); got != "[1,-2]" {
		t.Errorf("ints = %q", got)
	}
	if got := JSONList([]bool{true, false}).String(); got != "[true,false]" {
		t.Errorf("bools = %q", got)
	}
	if got := JSONList([]float64{1.5}).String(); got != "[1.5]" {
		t.Errorf("floats = %q", got)
	}
}

type siteTag struct{ name string }

func (s siteTag) Value() (driver.Value, error) { return "site:" + s.name, nil }

func TestJSONListValuer(t *testing.T) {
	if got := JSONList([]siteTag{{"alpha"}, {"beta"}}).String(); got != `["site:alpha","site:beta"]` {
		t.Errorf("valuers = %q, want the driver values", got)
	}
}

// TestJSONListRefusesWithoutPanicking proves a list Volt cannot bind is
// an error its statement returns, as an In over the same value is, not
// a panic where the statement is built (spec §predicates, §go-types).
func TestJSONListRefusesWithoutPanicking(t *testing.T) {
	far := time.Date(9999, 12, 31, 23, 0, 0, 0, time.FixedZone("", -5*3600))
	for name, l := range map[string]List{
		"year past 9999": JSONList([]time.Time{far}),
		"NaN":            JSONList([]float64{math.NaN()}),
	} {
		if _, err := l.Value(); err == nil {
			t.Errorf("%s: Value() has no error", name)
		}
		if _, err := ArgsBound([]any{1, l}); err == nil {
			t.Errorf("%s: ArgsBound bound it", name)
		}
	}
}

func TestJSONListOptional(t *testing.T) {
	absent, err := JSONListOptional(Null[[]string]{}).Value()
	if err != nil || absent != nil {
		t.Errorf("an absent list binds %#v, %v; want NULL", absent, err)
	}
	empty, err := JSONListOptional(Some([]string{})).Value()
	if err != nil || empty != "[]" {
		t.Errorf("a present empty list binds %#v, %v; want []", empty, err)
	}
	two, err := JSONListOptional(Some([]int{1, 2})).Value()
	if err != nil || two != "[1,2]" {
		t.Errorf("a present list binds %#v, %v; want [1,2]", two, err)
	}
	if s := JSONListOptional(Null[[]int]{}).String(); s != "NULL" {
		t.Errorf("an absent list reads %q, want NULL", s)
	}
}
