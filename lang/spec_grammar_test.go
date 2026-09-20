package lang

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/ebnf"
	"github.com/Piechutowski/volt/lang/parser"
)

// The specification's grammar is executable (D100): every EBNF block
// reads as ISO 14977, every name it uses is defined once, the collected
// grammars repeat the sections' definitions exactly, and the language
// the grammar defines agrees with the front end: every sentence derived
// from the grammar parses, and every text one token away from one is
// accepted by the front end exactly when the grammar accepts it.

type specBlock struct {
	line    int    // the first line of the block in spec.md
	heading string // the nearest heading above
	section string // the nearest ## heading above
	part    int    // 1 (schema core) or 2 (project and routing)
	g       *ebnf.Grammar
}

func (b specBlock) collected() bool { return strings.HasPrefix(b.section, "Collected grammar") }

// specGrammarBlocks reads every ```ebnf block of the spec.
func specGrammarBlocks(t *testing.T) []specBlock {
	t.Helper()
	lines := strings.Split(specText(t), "\n")
	part := 1
	heading, section := "", ""
	var blocks []specBlock
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "# Part II") {
			part = 2
		}
		if strings.HasPrefix(l, "#") {
			heading = strings.TrimLeft(l, "# ")
			if strings.HasPrefix(l, "## ") {
				section = heading
			}
		}
		if strings.TrimSpace(l) != "```ebnf" {
			continue
		}
		start := i + 1
		j := start
		for j < len(lines) && strings.TrimSpace(lines[j]) != "```" {
			j++
		}
		g, err := ebnf.Parse(strings.Join(lines[start:j], "\n"))
		if err != nil {
			t.Fatalf("docs/spec.md:%d (%s): the grammar block does not read: %v", start+1, heading, err)
		}
		blocks = append(blocks, specBlock{line: start + 1, heading: heading, section: section, part: part, g: g})
		i = j
	}
	return blocks
}

// specLanguage assembles the language of a .volt file from the
// sections' blocks: Part I's rules, then Part II's, which may only add
// names, except that §V12 redefines check (its first alternative is
// §6.6's), and the collected grammar of Part II extends element and
// setting value by reference. The lexical facts beside the grammar
// are the spec's prose rules, each cited.
func specLanguage(t *testing.T) *ebnf.Language {
	t.Helper()
	blocks := specGrammarBlocks(t)
	g := &ebnf.Grammar{}
	lexical := map[string]bool{}
	var va *specBlock
	for i, b := range blocks {
		if b.collected() {
			if b.part == 2 {
				va = &blocks[i]
			}
			continue
		}
		for _, r := range b.g.Rules {
			switch {
			case b.section == "Lexical Structure" || r.Name == "slash": // §3, and the one addition of §V0.2
				lexical[r.Name] = true
			}
			if b.part == 2 && r.Name == "check" {
				g.Set(r) // §V12 extends §6.6
				continue
			}
			if err := g.Add(r); err != nil {
				t.Fatalf("docs/spec.md:%d (%s): %v: a name is defined once, and a section that needs another's rule references it", b.line, b.heading, err)
			}
		}
	}
	if va == nil {
		t.Fatal("no collected grammar for Part II")
	}
	for _, name := range []string{"element", "setting value"} {
		ext, ok := va.g.Lookup(name).Expr.(ebnf.Alt)
		if !ok || len(ext.Items) < 2 || ebnf.Format(ext.Items[0]) != "" {
			t.Fatalf("Appendix VA: %s must extend Part I's by reference, an empty first alternative", name)
		}
		base := g.Lookup(name)
		g.Set(&ebnf.Rule{Name: name, Expr: ebnf.Alt{Items: append(alternatives(base.Expr), ext.Items[1:]...)}, Line: base.Line})
	}
	// A production that references sp explicitly lays out its own
	// whitespace (Notation, EBNF): nothing else lies between its symbols.
	contiguous := map[string]bool{
		"route path":   true, // §V4.1.1
		"package path": true, // §V2.2
		"param":        true, // §V10, the grammar's own note
	}
	for _, r := range g.Rules {
		for _, ref := range ebnf.Refs(r.Expr) {
			if ref == "sp" && !lexical[r.Name] {
				contiguous[r.Name] = true
			}
		}
	}
	oneOf := func(chars string) func(rune) bool { return func(r rune) bool { return strings.ContainsRune(chars, r) } }
	l := &ebnf.Language{
		G:          g,
		Start:      "program",
		Lexical:    lexical,
		Layout:     ebnf.Rep{X: ebnf.Alt{Items: []ebnf.Expr{ebnf.Ref{Name: "sp"}, ebnf.Ref{Name: "comment"}}}}, // §3.2 rule 3, §3.3 rule 4
		Newline:    "newline",                                                                                  // §3.2 rule 2
		Contiguous: contiguous,
		SameLine: map[string]bool{ // §4.2 rule 5
			"settings": true, "table settings": true, "column settings": true, "index settings": true,
			"check settings": true, "ref settings": true, "enum settings": true, "note settings": true,
			"table group settings": true,
		},
		Specials: map[string]func(rune) bool{
			"U+000A LINE FEED":      oneOf("\n"),
			"U+0020 SPACE":          oneOf(" "),
			"U+0009 TAB":            oneOf("\t"),
			"any Unicode character": func(rune) bool { return true },
			"any character of Unicode category L (Letter)": unicode.IsLetter,
			"any character of Unicode category M (Mark)":   unicode.IsMark,
		},
		Witness: map[string]string{
			"U+000A LINE FEED":      "\n",
			"U+0020 SPACE":          " ",
			"U+0009 TAB":            "\t",
			"any Unicode character": "a",
			"any character of Unicode category L (Letter)": "a",
			"any character of Unicode category M (Mark)":   "́",
		},
		Cut: true, // §3.2 rule 5
	}
	if err := l.Check(); err != nil {
		t.Fatalf("the spec's grammar is not closed:\n%v", err)
	}
	return l
}

func alternatives(e ebnf.Expr) []ebnf.Expr {
	if a, ok := e.(ebnf.Alt); ok {
		return a.Items
	}
	return []ebnf.Expr{e}
}

func TestSpecGrammarReadsAndIsClosed(t *testing.T) {
	specLanguage(t)
}

// The collected grammars are copies: Appendix IA repeats every rule
// of Part I's sections and nothing else; Appendix VA repeats every
// rule of Part II's sections but the query layer's (§V9 to §V12, which
// it says it leaves in place), extends element and setting value by
// reference, and defines nothing of its own.
func TestSpecCollectedGrammarsRepeatTheSections(t *testing.T) {
	blocks := specGrammarBlocks(t)
	sections := map[int]map[string]string{1: {}, 2: {}}
	where := map[string]string{}
	notCollected := map[string]bool{"Groups": true, "Predicates": true, "Selects over groups": true, "Validation checks": true}
	expected := map[int]map[string]bool{1: {}, 2: {}}
	for _, b := range blocks {
		if b.collected() {
			continue
		}
		for _, r := range b.g.Rules {
			sections[b.part][r.Name] = ebnf.Format(r.Expr)
			where[r.Name] = fmt.Sprintf("docs/spec.md:%d", b.line)
			if b.part == 1 || !notCollected[b.section] {
				expected[b.part][r.Name] = true
			}
		}
	}
	for _, b := range blocks {
		if !b.collected() {
			continue
		}
		seen := map[string]bool{}
		for _, r := range b.g.Rules {
			seen[r.Name] = true
			if b.part == 2 && (r.Name == "element" || r.Name == "setting value") {
				continue // extended by reference, checked by specLanguage
			}
			def, ok := sections[b.part][r.Name]
			if !ok {
				t.Errorf("%s (docs/spec.md:%d) defines %s, which no section of Part %d defines", b.section, b.line, r.Name, b.part)
				continue
			}
			if got := ebnf.Format(r.Expr); got != def {
				t.Errorf("%s repeats %s as\n\t%s\nbut %s defines it as\n\t%s", b.section, r.Name, got, where[r.Name], def)
			}
		}
		for name := range expected[b.part] {
			if !seen[name] {
				t.Errorf("%s (docs/spec.md:%d) lacks %s, defined at %s", b.section, b.line, name, where[name])
			}
		}
	}
}

// specSentences derives the grammar's sentences once per test binary.
func specSentences(t *testing.T) (*ebnf.Language, []ebnf.Sentence) {
	t.Helper()
	l := specLanguage(t)
	sentences, unreachable, err := l.Sentences()
	if err != nil {
		t.Fatal(err)
	}
	// Rules no program reaches: the layout rules, punct, which lists
	// the punctuation for the reader, and pred keyword, an exclusion
	// set referenced only on the right of an exception.
	want := "comment, line comment, block comment, block comment body, punct, pred keyword"
	if got := strings.Join(unreachable, ", "); got != want {
		t.Errorf("rules no sentence reaches: %s\nwant: %s", got, want)
	}
	return l, sentences
}

// sentencesDir, when set, receives every derived sentence as a .volt
// file, numbered, for the tree-sitter differential of docs/editor.md §8.
var sentencesDir = flag.String("sentences", "", "write the derived sentences of the spec's grammar to this directory")

// Every sentence the grammar derives is recognized by the grammar's
// own recognizer (the rendering and the recognizer agree on layout)
// and parses without a diagnostic.
func TestSpecGrammarSentencesParse(t *testing.T) {
	l, sentences := specSentences(t)
	if *sentencesDir != "" {
		if err := os.MkdirAll(*sentencesDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	failures := 0
	byRule := map[string]int{}
	for i, s := range sentences {
		text := ebnf.Text(s.Pieces)
		byRule[s.Rule]++
		if *sentencesDir != "" {
			name := fmt.Sprintf("%03d_%s.volt", i, strings.ReplaceAll(s.Rule, " ", "_"))
			if err := os.WriteFile(filepath.Join(*sentencesDir, name), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if !l.Accepts("program", text) {
			t.Errorf("%s: derived but not recognized: %q", s.Rule, text)
			failures++
		}
		if _, diags := parser.ParseFile("t.volt", text); len(diags) > 0 {
			if failures < 60 {
				t.Errorf("%s: the grammar derives %q, the parser says: %s", s.Rule, text, diags[0].Msg)
			}
			failures++
		}
	}
	if failures >= 60 {
		t.Errorf("%d failures in all", failures)
	}
	t.Logf("%d sentences over %d rules", len(sentences), len(byRule))
}

// specFrontEnd is the whole front end on one text: parse, load as a
// one-package project, check. What it rejects, the language rejects.
func specFrontEnd(t *testing.T) func(text string) bool {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ModFile), []byte("module corpus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "x.volt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return func(text string) bool {
		if _, diags := parser.ParseFile(path, text); len(diags) > 0 {
			return false
		}
		pr, err := LoadOverlay(root, map[string]string{path: text})
		if err != nil {
			return false
		}
		return !diag.HasErrors(Check(pr))
	}
}

// Every text one token away from a derived sentence (a token dropped,
// a token inserted, two adjacent tokens exchanged) is decided the same
// way by the grammar and by the front end: in the grammar, the parser
// accepts it; outside, the front end rejects it.
func TestSpecGrammarNeighboursAgreeWithTheFrontEnd(t *testing.T) {
	l, sentences := specSentences(t)
	front := specFrontEnd(t)
	inserted := []ebnf.Piece{{Text: "x"}, {Text: "1"}, {Text: ","}, {Text: "}"}, {Text: "\n", Sep: true}}
	seen := map[string]bool{}
	disagreements := 0
	checked := 0
	for _, s := range sentences {
		var near []string
		near = append(near, ebnf.Deletions(s.Pieces)...)
		near = append(near, ebnf.Insertions(s.Pieces, inserted)...)
		near = append(near, ebnf.Swaps(s.Pieces)...)
		for _, text := range near {
			if seen[text] {
				continue
			}
			seen[text] = true
			checked++
			inGrammar := l.Accepts("program", text)
			if inGrammar {
				if _, diags := parser.ParseFile("t.volt", text); len(diags) > 0 {
					if disagreements < 60 {
						t.Errorf("in the grammar, rejected by the parser: %q (from a sentence of %s): %s", text, s.Rule, diags[0].Msg)
					}
					disagreements++
				}
			} else if front(text) {
				if disagreements < 60 {
					t.Errorf("outside the grammar, accepted by the front end: %q (from a sentence of %s)", text, s.Rule)
				}
				disagreements++
			}
		}
	}
	if disagreements >= 60 {
		t.Errorf("%d disagreements in all", disagreements)
	}
	t.Logf("%d texts one token away from %d sentences checked", checked, len(sentences))
}
