package e2e

import (
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

type xtest struct {
	display uintptr

	flush    func(d uintptr) int32
	motion   func(d uintptr, screen int32, x, y int32, delay uint64) int32
	button   func(d uintptr, button uint32, press int32, delay uint64) int32
	key      func(d uintptr, code uint32, press int32, delay uint64) int32
	keysym   func(name string) uint64
	keycode  func(d uintptr, sym uint64) uint8
	closeDpy func(d uintptr) int32
}

func xtestOpen(t *testing.T, display string) *xtest {
	t.Helper()
	x11, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skip("synthetic input needs libX11:", err)
	}
	xtst, err := purego.Dlopen("libXtst.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skip("synthetic input needs libXtst:", err)
	}
	var open func(name string) uintptr
	x := &xtest{}
	purego.RegisterLibFunc(&open, x11, "XOpenDisplay")
	purego.RegisterLibFunc(&x.flush, x11, "XFlush")
	purego.RegisterLibFunc(&x.keysym, x11, "XStringToKeysym")
	purego.RegisterLibFunc(&x.keycode, x11, "XKeysymToKeycode")
	purego.RegisterLibFunc(&x.closeDpy, x11, "XCloseDisplay")
	purego.RegisterLibFunc(&x.motion, xtst, "XTestFakeMotionEvent")
	purego.RegisterLibFunc(&x.button, xtst, "XTestFakeButtonEvent")
	purego.RegisterLibFunc(&x.key, xtst, "XTestFakeKeyEvent")
	if x.display = open(display); x.display == 0 {
		t.Fatalf("XOpenDisplay(%s) failed", display)
	}
	t.Cleanup(func() { x.closeDpy(x.display) })
	return x
}

func (x *xtest) click(px, py int) {
	x.motion(x.display, -1, int32(px), int32(py), 0)
	x.flush(x.display)
	time.Sleep(80 * time.Millisecond)
	x.button(x.display, 1, 1, 0)
	x.flush(x.display)
	time.Sleep(80 * time.Millisecond)
	x.button(x.display, 1, 0, 0)
	x.flush(x.display)
	time.Sleep(80 * time.Millisecond)
}

func (x *xtest) press(name string) {
	code := uint32(x.keycode(x.display, x.keysym(name)))
	x.key(x.display, code, 1, 0)
	x.flush(x.display)
	time.Sleep(40 * time.Millisecond)
	x.key(x.display, code, 0, 0)
	x.flush(x.display)
	time.Sleep(40 * time.Millisecond)
}

func (x *xtest) typeText(text string) {
	for _, r := range text {
		name := string(r)
		if r == ' ' {
			name = "space"
		}
		x.press(name)
	}
}
