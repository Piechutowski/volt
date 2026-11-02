// Package disk is the one source.Source over the file system (spec
// §values-and-effects): the tool, the language server and the tests
// hand it to the loader, and no package under lang, gen or nao/gen
// imports os.
package disk

import (
	"os"

	"github.com/Piechutowski/volt/lang/source"
)

// Source reads the file system through os.
type Source struct{}

// ReadDir lists dir through os.ReadDir.
func (Source) ReadDir(dir string) ([]source.Entry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]source.Entry, len(entries))
	for i, e := range entries {
		out[i] = source.Entry{Name: e.Name(), Dir: e.IsDir()}
	}
	return out, nil
}

// ReadFile reads path through os.ReadFile.
func (Source) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// Exists reports whether os.Stat finds path.
func (Source) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
