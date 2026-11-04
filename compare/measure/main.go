// Measure counts what each stack of the comparison is made of, by one
// rule for all: lines that are neither blank nor comments, tests left
// out since every stack is held to the same suite. Hand-written Go and
// the contract or schema files it is written against are counted apart
// from generated code, the files carrying Go's generated marker, a
// typed client written by hand is counted on its own, and the
// third-party modules, the Volt framework among them, come from the
// stack's own import graph. Run
// it after go generate ./compare/...:
//
//	go run ./compare/measure
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type stack struct {
	dir, name string
}

var stacks = []stack{
	{"volt", "Volt"},
	{"gingorm", "Gin + GORM"},
	{"chisqlc", "chi + sqlc + goose"},
	{"echosqlx", "Echo + sqlx + goose"},
	{"oapient", "oapi-codegen + Ent"},
}

type count struct {
	handGo, client, contract, generated int
	files                               int
	modules                             []string
}

func main() {
	root, err := filepath.Abs(".")
	if err != nil {
		fail(err)
	}
	if _, err := os.Stat(filepath.Join(root, "compare", "suite")); err != nil {
		fail(fmt.Errorf("run from the repository root, after go generate ./compare/..."))
	}
	fmt.Println("| Stack | Server Go | Client Go | Contract and schema | Hand-written total | Generated | Third-party modules |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---:|")
	for _, s := range stacks {
		dir := filepath.Join(root, "compare", s.dir)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		c, err := measure(dir)
		if err != nil {
			fail(err)
		}
		if c.modules, err = modules(root, s.dir); err != nil {
			fail(err)
		}
		fmt.Printf("| %s | %d | %d | %d | %d | %d | %d |\n", s.name, c.handGo, c.client, c.contract, c.handGo+c.client+c.contract, c.generated, len(c.modules))
	}
	fmt.Println()
	for _, s := range stacks {
		if _, err := os.Stat(filepath.Join(root, "compare", s.dir)); err != nil {
			continue
		}
		mods, err := modules(root, s.dir)
		if err != nil {
			fail(err)
		}
		fmt.Printf("- %s: %s\n", s.name, strings.Join(mods, ", "))
	}
}

func measure(dir string) (count, error) {
	var c count
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch ext := filepath.Ext(name); {
		case strings.HasSuffix(name, "_test.go"):
		case ext == ".go" && generated(data):
			c.generated += lines(data, "//")
		case ext == ".go" && strings.Contains(filepath.ToSlash(path), "/ent/schema/"):
			c.contract += lines(data, "//")
		case ext == ".go" && strings.Contains(filepath.ToSlash(path), "client/"):
			c.client += lines(data, "//")
		case ext == ".go":
			c.handGo += lines(data, "//")
		case ext == ".sql" && generated(data):
			c.generated += lines(data, "--")
		case ext == ".sql":
			c.contract += lines(data, "--")
		case ext == ".volt":
			c.contract += lines(data, "//")
		case ext == ".yaml" || ext == ".yml":
			c.contract += lines(data, "#")
		default:
			return nil
		}
		c.files++
		return nil
	})
	return c, err
}

func generated(data []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "// Code generated ") || strings.HasPrefix(line, "-- Code generated ") {
			return strings.HasSuffix(strings.TrimSpace(line), "DO NOT EDIT.")
		}
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}

func lines(data []byte, comment string) int {
	n, block := 0, false
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, 1<<24)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case block:
			block = !strings.Contains(line, "*/")
		case line == "" || strings.HasPrefix(line, comment):
		case comment == "//" && strings.HasPrefix(line, "/*"):
			block = !strings.Contains(line, "*/")
		default:
			n++
		}
	}
	return n
}

func modules(root, dir string) ([]string, error) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}{{end}}", "./compare/"+dir+"/...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list for %s: %w", dir, err)
	}
	var mods []string
	for _, m := range strings.Fields(string(out)) {
		if strings.HasPrefix(m, "github.com/Piechutowski/volt/compare") || slices.Contains(mods, m) {
			continue
		}
		mods = append(mods, m)
	}
	slices.Sort(mods)
	return mods, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "measure:", err)
	os.Exit(1)
}
