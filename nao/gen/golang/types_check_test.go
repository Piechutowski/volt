package golang

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
)

// TestIntegerFamilyAgreesWithChecker pins lang/check's integer family
// (the §6.3 increment rule) to this package's type map: every DBML
// type mapping to a Go integer is an integer type there, and nothing
// else is — the two lists cannot drift apart.
func TestIntegerFamilyAgreesWithChecker(t *testing.T) {
	for dbml, gt := range typeMap {
		isInt := strings.HasPrefix(gt.name, "int") || strings.HasPrefix(gt.name, "uint")
		if got := check.IntegerType(dbml); got != isInt {
			t.Errorf("check.IntegerType(%q) = %v, but Appendix A maps it to %s", dbml, got, gt.name)
		}
	}
}

// TestRequiredKindAgreesWithTypeMap pins lang/check's required
// classification (§6.3 extension) to this package's type map: every
// string-mapped type is "text", every integer/unsigned/float type is
// "numeric", []byte and json.RawMessage are "bytes", and bool and
// time.Time cannot be required.
func TestRequiredKindAgreesWithTypeMap(t *testing.T) {
	for dbml, gt := range typeMap {
		want, ok := "", true
		switch {
		case gt.name == "string":
			want = "text"
		case strings.HasPrefix(gt.name, "int"), strings.HasPrefix(gt.name, "uint"), strings.HasPrefix(gt.name, "float"):
			want = "numeric"
		case gt.name == "[]byte", gt.name == "json.RawMessage":
			want = "bytes"
		default:
			ok = false
		}
		got, gotOK := check.RequiredKind(dbml, false)
		if gotOK != ok || got != want {
			t.Errorf("check.RequiredKind(%q) = (%q, %v), but Appendix A maps it to %s", dbml, got, gotOK, gt.name)
		}
	}
}
