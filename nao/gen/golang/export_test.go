package golang

import "testing"

// GoldenCompare is compareGolden for the external test package, which
// lowers selects through lang before generating nao_selects.go
// (spec §selects-over-groups).
func GoldenCompare(t *testing.T, got []byte, golden string) {
	t.Helper()
	compareGolden(t, got, golden)
}
