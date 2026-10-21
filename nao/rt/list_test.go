package rt

import (
	"database/sql/driver"
	"testing"
)

func TestJSONList(t *testing.T) {
	if got := JSONList[string](nil); got != "[]" {
		t.Errorf("nil = %q, want []", got)
	}
	if got := JSONList([]string{"a", `b"c`}); got != `["a","b\"c"]` {
		t.Errorf("strings = %q", got)
	}
	if got := JSONList([]int32{1, -2}); got != "[1,-2]" {
		t.Errorf("ints = %q", got)
	}
	if got := JSONList([]bool{true, false}); got != "[true,false]" {
		t.Errorf("bools = %q", got)
	}
	if got := JSONList([]float64{1.5}); got != "[1.5]" {
		t.Errorf("floats = %q", got)
	}
}

type siteTag struct{ name string }

func (s siteTag) Value() (driver.Value, error) { return "site:" + s.name, nil }

func TestJSONListValuer(t *testing.T) {
	if got := JSONList([]siteTag{{"alpha"}, {"beta"}}); got != `["site:alpha","site:beta"]` {
		t.Errorf("valuers = %q, want the driver values", got)
	}
}
