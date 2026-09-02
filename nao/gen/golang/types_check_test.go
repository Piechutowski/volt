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
