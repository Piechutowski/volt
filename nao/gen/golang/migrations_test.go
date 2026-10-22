package golang

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestMigrationsGolden(t *testing.T) {
	fixture := filepath.Join("..", "..", "migrate", "testdata", "blog.volt")
	f, info := parseChecked(t, fixture)
	out, err := GenerateMigrations(f, info, Options{Package: "models", Source: filepath.Base(fixture)})
	if err != nil {
		t.Fatalf("GenerateMigrations: %v", err)
	}
	compareGolden(t, out, filepath.Join("testdata", "blog_migrations.go.golden"))
	for _, want := range []string{`"context"`, `"database/sql"`, `"github.com/Piechutowski/volt/nao/rt"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing import %s", want)
		}
	}
	if strings.Count(string(out), "\n\t\"") != 3 {
		t.Errorf("volt_migrations.go imports context, database/sql and rt only:\n%s", out)
	}
}

func TestMigrationsEmptyAndBackquoted(t *testing.T) {
	src := "package db\n\nTable t {\n  id int [pk, increment]\n}\n\n" +
		"Migration 0001__init {\n}\n\n" +
		"Migration 0002__tick [no_transaction] {\n  up {\n    exec 'SELECT `x`'\n  }\n}\n"
	f, diags := parser.ParseFile("t.volt", src)
	info, semDiags := check.File(f)
	if diag.HasErrors(append(diags, semDiags...)) {
		t.Fatalf("input invalid: %v %v", diags, semDiags)
	}
	out, err := GenerateMigrations(f, info, Options{Package: "db", Source: "t.volt"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"\t\tName: \"0001__init\",\n\t\tUp:   []string{},\n\t\tDown: []string{},\n",
		"\t\tName: \"0002__tick\",\n\t\tUp: []string{\n\t\t\t\"SELECT `x`\",\n\t\t},\n\t\tNoTransaction: true,\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks\n%s\n--- got ---\n%s", want, got)
		}
	}
	if !gofmtStable(t, out) {
		t.Errorf("output is not gofmt-canonical:\n%s", got)
	}
}
