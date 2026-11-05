package lsp

import (
	"testing"
	"unsafe"
)

func TestVoltDefStoredInline(t *testing.T) {
	if size := unsafe.Sizeof(voltDef{}); size > 128 {
		t.Errorf("voltDef is %d bytes: a map stores a value over 128 bytes behind a pointer, one allocation per symbol on every index build; keep rare facts in a side map as relationColumns does", size)
	}
}
