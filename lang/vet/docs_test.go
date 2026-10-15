package vet_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/vet"
)

func TestRulesDocumentation(t *testing.T) {
	docBytes, err := os.ReadFile(filepath.Join("..", "..", "docs", "lint.md"))
	if err != nil {
		t.Fatalf("docs/lint.md: %v", err)
	}
	doc := string(docBytes)

	headingRE := regexp.MustCompile(`(?m)^### ([a-z]+)$`)
	linkRE := regexp.MustCompile(`\(\.\./lang/vet/(testdata/[a-z_]+\.(?:dbml|volt))\)`)

	headings := headingRE.FindAllStringSubmatchIndex(doc, -1)
	sections := map[string]string{}
	var listed []string
	for i, h := range headings {
		name := doc[h[2]:h[3]]
		listed = append(listed, name)
		end := len(doc)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		if _, dup := sections[name]; dup {
			t.Errorf("lint.md: duplicate section for %q", name)
		}
		sections[name] = doc[h[1]:end]
	}

	registered := map[string]bool{}
	var order []string
	for _, a := range vet.All() {
		registered[a.Name()] = true
		order = append(order, a.Name())
	}
	if strings.Join(order, ",") != strings.Join(listed, ",") {
		t.Errorf("lint.md lists the rules as %v; vet.All() registers %v", listed, order)
	}

	for name := range sections {
		if !registered[name] {
			t.Errorf("lint.md documents %q, which is not a registered analyzer", name)
		}
	}

	linkedAnywhere := map[string]bool{}
	for _, a := range vet.All() {
		sec, ok := sections[a.Name()]
		if !ok {
			t.Errorf("analyzer %q has no '### %s' section in lint.md", a.Name(), a.Name())
			continue
		}
		links := linkRE.FindAllStringSubmatch(sec, -1)
		if len(links) == 0 {
			t.Errorf("lint.md section %q links no testdata file", a.Name())
			continue
		}
		ranByLinked, wantInLinked := false, false
		for _, m := range links {
			rel := m[1]
			linkedAnywhere[filepath.Base(rel)] = true
			src, err := os.ReadFile(rel)
			if err != nil {
				t.Errorf("lint.md section %q links %s: %v", a.Name(), rel, err)
				continue
			}
			lines := strings.Split(string(src), "\n")
			header := strings.TrimPrefix(strings.TrimSpace(lines[0]), "// analyzers:")
			for _, n := range strings.Split(header, ",") {
				if strings.TrimSpace(n) == a.Name() {
					ranByLinked = true
				}
			}
			if strings.Contains(string(src), "//WANT") && containsWant(string(src), a.Name()) {
				wantInLinked = true
			}
		}
		if !ranByLinked {
			t.Errorf("no file linked from lint.md section %q runs the analyzer (missing from '// analyzers:' header)", a.Name())
		}
		if !wantInLinked {
			t.Errorf("no file linked from lint.md section %q contains a bad example ('//WANT %s')", a.Name(), a.Name())
		}
	}

	files, err := filepath.Glob(filepath.Join("testdata", "*.dbml"))
	if err != nil {
		t.Fatal(err)
	}
	extended, err := filepath.Glob(filepath.Join("testdata", "*.volt"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, extended...)
	if len(files) == 0 {
		t.Fatal("no testdata files")
	}
	for _, f := range files {
		if !linkedAnywhere[filepath.Base(f)] {
			t.Errorf("testdata file %s is not linked from lint.md", f)
		}
	}
}

func containsWant(src, name string) bool {
	for _, m := range regexp.MustCompile(`//WANT ([a-z,]+)`).FindAllStringSubmatch(src, -1) {
		for _, n := range strings.Split(m[1], ",") {
			if n == name {
				return true
			}
		}
	}
	return false
}
