package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	assertDocPath        = "../../docs/assert.md"
	assertDocPackagePath = "../../assert"
	assertDocCheckedPath = "../../checked"
	assertDocGuiguiPath  = "testdata/guigui"
	assertDocGuiguiPkg   = "github.com/guigui-gui/guigui"
	assertDocPlain       = "example"
	assertDocGuigui      = "widget"
)

var assertDocInventory = map[string]bool{
	assertDocPlain + ".pool": true,
}

type assertDocBlock struct {
	kind string
	text string
	line int
}

type assertDocImporter struct {
	loaded   map[string]*types.Package
	fallback types.Importer
}

func (imported assertDocImporter) Import(path string) (*types.Package, error) {
	if found, ok := imported.loaded[path]; ok {
		return found, nil
	}
	return imported.fallback.Import(path)
}

func TestAssertDoc(t *testing.T) {
	text, err := os.ReadFile(assertDocPath)
	if err != nil {
		t.Fatal(err)
	}
	blocks := assertDocBlocks(t, string(text))
	fileSet := token.NewFileSet()
	imported := docImporterNew(t)
	for _, kind := range []string{assertDocPlain, assertDocGuigui} {
		var selected []assertDocBlock
		for _, block := range blocks {
			if block.kind == kind {
				selected = append(selected, block)
			}
		}
		if len(selected) == 0 {
			t.Errorf("docs/assert.md has no %s example", kind)
			continue
		}
		for _, finding := range assertDocFindings(t, fileSet, imported, kind, selected) {
			t.Errorf("the style gate fails an example of docs/assert.md: %s %s %d",
				finding.rule, finding.key, finding.value)
		}
	}
}

func docImporterNew(t *testing.T) assertDocImporter {
	t.Helper()
	fileSet := token.NewFileSet()
	imported := assertDocImporter{loaded: map[string]*types.Package{}, fallback: importer.Default()}
	for directory, path := range map[string]string{
		assertDocPackagePath: module + "/assert",
		assertDocCheckedPath: module + "/checked",
		assertDocGuiguiPath:  assertDocGuiguiPkg,
	} {
		imported.loaded[path] = assertDocDirectoryCheck(t, fileSet, imported, directory, path)
	}
	return imported
}

func assertDocBlocks(t *testing.T, doc string) []assertDocBlock {
	t.Helper()
	var blocks []assertDocBlock
	var current *assertDocBlock
	for number, line := range strings.Split(doc, "\n") {
		fields := strings.Fields(line)
		switch {
		case current == nil && len(fields) > 0 && strings.HasPrefix(fields[0], "```go"):
			kind := strings.Join(fields, " ")
			switch kind {
			case "```go":
				current = &assertDocBlock{kind: assertDocPlain, line: number + 1}
			case "```go guigui":
				current = &assertDocBlock{kind: assertDocGuigui, line: number + 1}
			default:
				t.Errorf("docs/assert.md:%d: a Go block opens with ```go or ```go guigui, not %s",
					number+1, kind)
			}
		case current != nil && strings.TrimSpace(line) == "```":
			blocks = append(blocks, *current)
			current = nil
		case current != nil:
			current.text += styleDocLineTabbed(line) + "\n"
		}
	}
	return blocks
}

func assertDocDirectoryCheck(
	t *testing.T, fileSet *token.FileSet, imported assertDocImporter, directory string, path string,
) *types.Package {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, source := range paths {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, source, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	config := types.Config{Importer: imported}
	checked, err := config.Check(path, fileSet, files, nil)
	if err != nil {
		t.Fatalf("%s: %v", directory, err)
	}
	return checked
}

func assertDocFindings(
	t *testing.T, fileSet *token.FileSet, imported assertDocImporter, kind string, blocks []assertDocBlock,
) []styleFinding {
	t.Helper()
	var files []*ast.File
	var names []string
	var findings []styleFinding
	for _, block := range blocks {
		name := fmt.Sprintf("docs/assert.md:%d", block.line)
		text := "package " + kind + "\n\n" + block.text
		formatted, err := format.Source([]byte(text))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(formatted) != text {
			t.Errorf("%s: the example is not formatted by gofmt", name)
		}
		located := fmt.Sprintf("package %s\n//line docs/assert.md:%d\n%s", kind, block.line+1, block.text)
		file, err := parser.ParseFile(fileSet, name, located, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			t.Fatalf("%v", err)
		}
		files = append(files, file)
		names = append(names, name)
		findings = append(findings, styleLineFindings(styleSource{name, []byte(block.text)})...)
	}
	information := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	config := types.Config{Importer: imported}
	checked, err := config.Check(kind, fileSet, files, information)
	if err != nil {
		t.Fatalf("the %s examples of docs/assert.md: %v", kind, err)
	}
	var typed []styleTyped
	for index, file := range files {
		units := styleUnits(names[index], file)
		findings = append(findings, styleUnitFindings(fileSet, units)...)
		typed = append(typed, styleTyped{names[index], file, information, checked, units})
	}
	for _, file := range typed {
		findings = append(findings, styleTypedFindings(file)...)
	}
	return append(findings, styleRecursionFindings(styleGraphBuild(typed))...)
}
