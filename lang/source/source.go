// Package source is where a project's files come from (spec
// §values-and-effects): the loader reads directories and files through
// a Source the tool or the language server hands it, so the front end
// imports nothing that touches the world.
package source

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is one directory entry: its name and whether it is a directory.
type Entry struct {
	Name string
	Dir  bool
}

// Source reads a tree of files by absolute path (spec
// §values-and-effects). ReadDir lists a directory's entries in name
// order; ReadFile reads one file; Exists reports whether a path names
// a file or a directory.
type Source interface {
	ReadDir(dir string) ([]Entry, error)
	ReadFile(path string) ([]byte, error)
	Exists(path string) bool
}

// Map is a Source over an in-memory map from absolute file path to
// content, for tests: directories are implied by the paths.
type Map map[string]string

// ErrNotExist is what Map reports for a path it does not hold.
var ErrNotExist = errors.New("source: no such file or directory")

// ReadDir lists the files and directories directly under dir.
func (m Map) ReadDir(dir string) ([]Entry, error) {
	dir = filepath.Clean(dir)
	seen := map[string]bool{}
	var out []Entry
	for path := range m {
		rel, err := filepath.Rel(dir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		first, rest, found := strings.Cut(rel, string(filepath.Separator))
		if seen[first] {
			continue
		}
		seen[first] = true
		out = append(out, Entry{Name: first, Dir: found && rest != ""})
	}
	if len(out) == 0 && !m.Exists(dir) {
		return nil, ErrNotExist
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ReadFile reads one file of the map.
func (m Map) ReadFile(path string) ([]byte, error) {
	text, ok := m[filepath.Clean(path)]
	if !ok {
		return nil, ErrNotExist
	}
	return []byte(text), nil
}

// Exists reports whether path is a file of the map or a directory one
// of its files lies under.
func (m Map) Exists(path string) bool {
	path = filepath.Clean(path)
	if _, ok := m[path]; ok {
		return true
	}
	prefix := path + string(filepath.Separator)
	for p := range m {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
