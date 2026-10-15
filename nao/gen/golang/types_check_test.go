package golang

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
)

func TestIntegerFamilyAgreesWithChecker(t *testing.T) {
	for dbml, gt := range typeMap {
		isInt := strings.HasPrefix(gt.name, "int") || strings.HasPrefix(gt.name, "uint")
		if got := check.IntegerType(dbml); got != isInt {
			t.Errorf("check.IntegerType(%q) = %v, but §mapping-to-go maps it to %s", dbml, got, gt.name)
		}
	}
}

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
			t.Errorf("check.RequiredKind(%q) = (%q, %v), but §mapping-to-go maps it to %s", dbml, got, gotOK, gt.name)
		}
	}
}
