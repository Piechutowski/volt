package source

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestMapSource(t *testing.T) {
	root := filepath.FromSlash("/p")
	m := Map{
		filepath.Join(root, "go.mod"):              "module m",
		filepath.Join(root, "db", "schema.volt"):   "package db",
		filepath.Join(root, "db", "sub", "x.volt"): "package sub",
	}
	entries, err := m.ReadDir(filepath.Join(root, "db"))
	if err != nil || !reflect.DeepEqual(entries, []Entry{{Name: "schema.volt"}, {Name: "sub", Dir: true}}) {
		t.Errorf("ReadDir = %v, %v", entries, err)
	}
	if _, err := m.ReadDir(filepath.Join(root, "nope")); err == nil {
		t.Error("ReadDir of a missing directory did not fail")
	}
	if b, err := m.ReadFile(filepath.Join(root, "go.mod")); err != nil || string(b) != "module m" {
		t.Errorf("ReadFile = %q, %v", b, err)
	}
	if !m.Exists(filepath.Join(root, "db")) || !m.Exists(filepath.Join(root, "go.mod")) || m.Exists(filepath.Join(root, "db", "nope")) {
		t.Error("Exists disagrees with the map")
	}
}
