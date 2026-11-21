package lsp

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Piechutowski/volt/lang/scanner"
	"github.com/Piechutowski/volt/lsp/protocol"
)

func TestIdentClassesAgreeWithTheScanner(t *testing.T) {
	start := regexp.MustCompile(`^` + identStart + `$`)
	char := regexp.MustCompile(`^` + identChar + `$`)
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		s := string(r)
		if start.MatchString(s) != scanner.IsIdentStart(r) || char.MatchString(s) != scanner.IsIdentChar(r) {
			t.Fatalf("%U: the classes say start %v char %v, the scanner start %v char %v",
				r, start.MatchString(s), char.MatchString(s), scanner.IsIdentStart(r), scanner.IsIdentChar(r))
		}
	}
}

const identSchema = "package db\n\n" +
	"table pósts {\n\tid    int [pk]\n\ttítle string\n}\n\n" +
	"table tägs {\n\tpóst_id int    [pk]\n\ttag     string [pk]\n}\n\n" +
	"ref: tägs.póst_id > pósts.id [as: póst, reverse: tägs]\n\n" +
	"pred récent { id > 0 }\n\n" +
	"select listé for pósts where récent\n"

func TestCompletionNonASCIINames(t *testing.T) {
	root := voltProject(t, map[string]string{"go.mod": "module ident\n", "db/schema.volt": identSchema})
	for _, tc := range []struct{ to, want string }{
		{"listé for pósts where ré", "récent"},
		{"listé for pó", "pósts"},
		{"listé for pósts where exists tä", "tägs"},
	} {
		text := strings.Replace(identSchema, "listé for pósts where récent", tc.to, 1)
		d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
		pos := offsetToLSP(text, strings.Index(text, tc.to)+len(tc.to))
		if items := d.Complete(pos); !hasLabel(items, tc.want) {
			t.Errorf("completion after %q = %v, want %s", tc.to, labels(items), tc.want)
		}
	}
}

func TestIdentSchemaChecksClean(t *testing.T) {
	root := voltProject(t, map[string]string{"go.mod": "module ident\n", "db/schema.volt": identSchema})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), identSchema)
	if len(d.Diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", d.Diags)
	}
}

func TestQualifiedReferenceTakesNonASCIINames(t *testing.T) {
	if m := voltQualRefRE.FindStringSubmatch("\tload x dé.pó"); m == nil || m[1] != "dé" || m[2] != "pó" {
		t.Errorf("the qualified reference of dé.pó is %q, want dé and pó", m)
	}
}

func TestRenameToANonASCIINameBesideAnError(t *testing.T) {
	text := "table posts {\n\tid int [pk]\n}\n\nref: posts.missing > posts.id\n"
	d := doc(t, text)
	if len(d.Diags) == 0 {
		t.Fatal("the fixture has no error to keep")
	}
	if _, err := d.Rename(posOf(t, text, "posts", 0), "pósté"); err != nil {
		t.Errorf("a rename that adds no error is refused: %v", err)
	}
}

func TestDiagnosticRangeOfANonIdentifierRune(t *testing.T) {
	text := "table posts {\n\tid int [pk]\n\ttitle—x string\n}\n"
	d := doc(t, text)
	at := posOf(t, text, "—", 0)
	at.Character = protocol.UInteger(len("\ttitle"))
	for _, dg := range d.LSPDiagnostics() {
		if dg.Range.Start == at && dg.Range.End.Character != at.Character+1 {
			t.Errorf("the diagnostic at — spans %+v, want one UTF-16 unit", dg.Range)
		}
	}
}
