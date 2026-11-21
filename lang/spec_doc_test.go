package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

var (
	headingRE = regexp.MustCompile(`(?m)^(#{1,3}) (.+)$`)
	tocLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`)

	docCiteRE       = regexp.MustCompile(`\b(` + citedDocs + `)/([A-Za-z0-9][A-Za-z0-9-]*)`)
	docNumberedRE   = regexp.MustCompile(numberedWord + `|` + appendixWord + `|` + numberedCite)
	docRuleIDRE     = regexp.MustCompile(`\*\*[A-Z][0-9]+\.|` + ruleIDCite)
	specRuleRE      = regexp.MustCompile(`\bspec/[a-z][a-z0-9-]*,?` + ruleNumber)
	commentLeadRE   = regexp.MustCompile(`\n[\s\p{Zs}]*(?://|#)?[\s\p{Zs}]*`)
	specRFCRE       = regexp.MustCompile(`RFC[\s\p{Zs}]+[0-9]+(?:` + rfcSep + rfcRef + `)*` + rfcSep + `$`)
	mdAnchorLinkRE  = regexp.MustCompile(`\]\(([A-Za-z0-9_./-]*\.md)?#([^)\s]+)(?:\s+` + mdTitle + `)?\)`)
	mdRefDefRE      = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:\s*([A-Za-z0-9_./-]*\.md)?#(\S+)(?:\s+` + mdTitle + `)?\s*$`)
	mdHeadingRE     = regexp.MustCompile(`^#{1,6} (.+)$`)
	numberedTitleRE = regexp.MustCompile(`^(?:[0-9]|` + numberedTitle + `)`)
)

const (
	citedDocs     = `spec|volt_style|style_in_volt|assert|plan|runner|editor|lint|roadmap`
	numberedWord  = `\b(?:[Ss]ections?|[Ss]teps?|[Pp]arts?)[\s\p{Zs}]+[0-9]`
	appendixWord  = `\bAppendix[\s\p{Zs}]+[A-Z]+\b`
	numberedCite  = `\b(?:` + citedDocs + `)/[A-Z]*[0-9]`
	ruleIDCite    = `\b(?:` + citedDocs + `|VOLT_STYLE)\.md` + "`?" + `(?:'s)?[\s\p{Zs}]+[A-Z][0-9]+\b`
	ruleNumber    = `[\s\p{Zs}]+(?:rule|clause)s?(?:[\s\p{Zs}]+[0-9]|[\s\p{Zs}]*$)`
	numberedTitle = `(?:Steps?|Parts?|Sections?)[\s\p{Zs}]+[0-9]|Appendix [A-Z]+[:.]|[A-C]\.[0-9]`
	sectionSign   = '\u00a7'
	rfcSep        = `[\s\p{Zs},;()\x{2013}-]*(?:(?:and|or)[\s\p{Zs}(]*)?`
	rfcRef        = `sections?[\s\p{Zs}]+[0-9][0-9.]*(?:[\s\p{Zs}]*[\x{2013}-][\s\p{Zs}]*[0-9][0-9.]*)?`
	mdTitle       = `(?:"[^"]*"|'[^']*'|\([^)]*\))`
)

func docFile(doc string) string {
	if doc == "volt_style" {
		return "VOLT_STYLE.md"
	}
	return doc + ".md"
}

func specText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "docs", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func anchorOf(title string) string {
	a := strings.ToLower(title)
	var b strings.Builder
	for _, r := range a {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), "-")
}

func specHeadings(spec string) [][]string {
	return headingRE.FindAllStringSubmatch(strings.Join(mdProse(spec), "\n"), -1)
}

func specAnchorsOf(spec string) map[string]bool {
	anchors := map[string]bool{}
	for _, m := range specHeadings(spec) {
		anchors[anchorOf(strings.TrimSpace(m[2]))] = true
	}
	return anchors
}

func specAnchors(t *testing.T) map[string]bool {
	t.Helper()
	anchors := specAnchorsOf(specText(t))
	if len(anchors) < 60 {
		t.Fatalf("spec looks truncated: %d headings", len(anchors))
	}
	return anchors
}

func repoTextFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "node_modules", "target", "grammars":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(d.Name()) {
		case ".go", ".volt", ".md", ".js", ".scm", ".golden", ".sql", ".sh", ".toml", ".rs", ".txt":
			files = append(files, path)
		}
		return nil
	})
	return files
}

func proseFiles(t *testing.T) []string {
	t.Helper()
	docs, err := filepath.Glob(filepath.Join("..", "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	return append(docs, filepath.Join("..", "README.md"), filepath.Join("..", "AGENT.md"))
}

func TestDocHeadingsUnnumberedAndUnique(t *testing.T) {
	for _, path := range proseFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, line := range mdProse(string(b)) {
			m := mdHeadingRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			title := strings.TrimSpace(m[1])
			if numberedTitleRE.MatchString(title) {
				t.Errorf("%s: numbered heading: %q", path, title)
			}
			anchor := githubAnchorOf(title)
			if strings.HasPrefix(line, "# ") || title == "Table of Contents" {
				continue
			}
			if seen[anchor] {
				t.Errorf("%s: duplicate heading %q: anchors would collide", path, title)
			}
			seen[anchor] = true
		}
	}
}

func TestNumberedTitlesCaught(t *testing.T) {
	for title, want := range map[string]bool{
		"1. Values":                        true,
		"St" + "ep 22. The assert package": true,
		"St" + "eps 22 to 35. Paying off":  true,
		"Pa" + "rt 0. Decisions":           true,
		"Append" + "ix A: Grammar":         true,
		"B.2 Lexical":                      true,
		"The assert package":               false,
		"Short lines":                      false,
		"`volttest`":                       false,
	} {
		if got := numberedTitleRE.MatchString(title); got != want {
			t.Errorf("numberedTitleRE on %q = %v, want %v", title, got, want)
		}
	}
}

func TestSpecTableOfContentsMatchesHeadings(t *testing.T) {
	spec := specText(t)
	start := strings.Index(spec, "## Table of Contents")
	end := strings.Index(spec[start+5:], "\n## ") + start + 5
	toc := spec[start:end]
	var listed []string
	for _, m := range tocLinkRE.FindAllStringSubmatch(toc, -1) {
		listed = append(listed, m[1])
		if m[2] != anchorOf(m[1]) {
			t.Errorf("ToC link for %q points at #%s, want #%s", m[1], m[2], anchorOf(m[1]))
		}
	}
	var actual []string
	for _, m := range specHeadings(spec) {
		title := strings.TrimSpace(m[2])
		if len(m[1]) >= 2 && title != "Table of Contents" {
			actual = append(actual, title)
		}
	}
	if strings.Join(listed, "\n") != strings.Join(actual, "\n") {
		t.Errorf("table of contents drifted from the headings\nToC:\n%s\n\nHeadings:\n%s", strings.Join(listed, "\n"), strings.Join(actual, "\n"))
	}
}

// Every citation and every diagnostic code in the repository names a heading
// of the doc it cites (decision named-sections).
func TestDocCitationsResolve(t *testing.T) {
	anchors := map[string]map[string]bool{"spec": specAnchors(t)}
	for _, doc := range strings.Split(citedDocs, "|") {
		if doc == "spec" {
			continue
		}
		b, err := os.ReadFile(filepath.Join("..", "docs", docFile(doc)))
		if err != nil {
			t.Fatal(err)
		}
		anchors[doc] = mdAnchors(string(b))
	}
	var bad []string
	for _, path := range repoTextFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, m := range docCiteRE.FindAllStringSubmatch(string(b), -1) {
			if !anchors[m[1]][m[2]] {
				bad = append(bad, path+": "+m[0])
			}
		}
	}
	if len(bad) > 0 {
		if len(bad) > 40 {
			bad = append(bad[:40], "…")
		}
		t.Errorf("citations that resolve to no heading:\n%s", strings.Join(bad, "\n"))
	}
}

func TestDocCitesParsed(t *testing.T) {
	for line, want := range map[string]string{
		"see spec/columns":                           "spec/columns",
		"see spec/types, spec/doc-comments":          "spec/types,spec/doc-comments",
		"(volt_style/owned-state-only, plan/limits)": "volt_style/owned-state-only,plan/limits",
		"runner/two-pools and editor/the-grammar":    "runner/two-pools,editor/the-grammar",
		"lint/unusedpartial, roadmap/roadmap":        "lint/unusedpartial,roadmap/roadmap",
		"docs/specs/y and nao/run/z":                 "",
	} {
		var got []string
		for _, m := range docCiteRE.FindAllStringSubmatch(line, -1) {
			got = append(got, m[1]+"/"+m[2])
		}
		if strings.Join(got, ",") != want {
			t.Errorf("docCiteRE on %q = %q, want %q", line, got, want)
		}
	}
}

func TestNoSectionSign(t *testing.T) {
	var bad []string
	for _, path := range repoTextFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.ContainsRune(line, sectionSign) {
				bad = append(bad, fmt.Sprintf("%s:%d", path, i+1))
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("a citation is <doc>/<anchor>, never the section sign:\n%s",
			strings.Join(bad, "\n"))
	}
}

func numberedCitations(line string) []string {
	var out []string
	for _, loc := range docNumberedRE.FindAllStringIndex(line, -1) {
		if specRFCRE.MatchString(line[:loc[0]]) {
			continue
		}
		out = append(out, line[loc[0]:loc[1]])
	}
	for _, loc := range docRuleIDRE.FindAllStringIndex(line, -1) {
		out = append(out, line[loc[0]:loc[1]])
	}
	return out
}

func TestNumberedCitationsCaught(t *testing.T) {
	sect, step, part, nbsp, nnbsp := "sect"+"ion", "st"+"ep", "pa"+"rt", "\u00a0", "\u202f"
	for _, c := range []struct{ line, want string }{
		{"see " + sect + " 6.3", sect + " 6"},
		{"see " + sect + nbsp + "6.3", sect + nbsp + "6"},
		{"see " + sect + nnbsp + "6.3", sect + nnbsp + "6"},
		{"see " + sect + "s 6 and 7", sect + "s 6"},
		{"S" + sect[1:] + " 8's checks", "S" + sect[1:] + " 8"},
		{"`plan.md` " + step + " 22", step + " 2"},
		{"until " + step + "s 22 to 35 land", step + "s 2"},
		{"read " + part + " 0 first", part + " 0"},
		{"in Append" + "ix B", "Append" + "ix B"},
		{"in Append" + "ix VA, collected", "Append" + "ix VA"},
		{"in Append" + "ix" + nbsp + "VA", "Append" + "ix" + nbsp + "VA"},
		{"cites spec/" + "V4", "spec/" + "V4"},
		{"cites volt_style/" + "4", "volt_style/" + "4"},
		{"`VOLT_STYLE.md`" + " S5 asks", "VOLT_STYLE.md`" + " S5"},
		{"VOLT_STYLE.md" + "'s F4 inventory", "VOLT_STYLE.md" + "'s F4"},
		{"- **" + "F1. Values.**", "**" + "F1."},
		{"a date per RFC 3339 " + sect + " 5.6", ""},
		{"a URI per RFC 3986 Append" + "ix A", ""},
		{"RFC 3339 " + sect + " 5.6 but " + sect + " 2", sect + " 2"},
		{"a date per RFC 3339, " + sect + " 5.6", ""},
		{"a date per RFC" + nbsp + "3339 " + sect + " 5.6", ""},
		{"per RFC 9110 " + sect + "s 15.5 and 15.6", ""},
		{"per RFC 9110 " + sect + " 15.5 and " + sect + " 15.6", ""},
		{"per RFC 3339 (" + sect + " 5.6)", ""},
		{"per RFC 9110 (" + sect + " 15.5, " + sect + " 15.6) or " + sect + " 9", ""},
		{"per RFC 9110 " + sect + " 15.5 or " + sect + " 15.6, then " + step + " 4", step + " 4"},
		{"per RFC 3339 " + sect + " 5.6. See " + sect + " 2", sect + " 2"},
		{"cites spec/columns and volt_style/short-functions", ""},
		{"see the Appendices, the " + sect + " on names, the next " + step + ", V4 ids", ""},
	} {
		got := strings.Join(numberedCitations(c.line), ",")
		if got != c.want {
			t.Errorf("numberedCitations(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestRuleCitationsCaught(t *testing.T) {
	sp, nbsp := "spec/", "\u00a0"
	for line, want := range map[string]bool{
		"see " + sp + "columns rule 2":               true,
		"see " + sp + "migrations rules 8 and 12":    true,
		"see " + sp + "table-resolution clause 2":    true,
		"see " + sp + "predicates" + nbsp + "rule 6": true,
		"(" + sp + "predicates rule":                 true,
		"see " + sp + "columns, rule 2":              true,
		"(" + sp + "columns\n// rule 2)":             true,
		"(" + sp + "columns,\n\t// rule 2)":          true,
		"see " + sp + "columns and the rule":         false,
		"the rules of " + sp + "indexes":             false,
	} {
		if got := specRuleRE.MatchString(commentsJoined(line)); got != want {
			t.Errorf("specRuleRE on %q = %v, want %v", line, got, want)
		}
	}
}

// commentsJoined is text with each line break and the comment leader
// after it folded into one space, so a citation broken across the lines
// of a comment reads as one line does.
func commentsJoined(text string) string {
	return commentLeadRE.ReplaceAllString(text, " ")
}

func TestCitationsNeverNumbered(t *testing.T) {
	var bad []string
	for _, path := range repoTextFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, m := range numberedCitations(line) {
				bad = append(bad, fmt.Sprintf("%s:%d: %q", path, i+1, m))
			}
		}
		for _, m := range specRuleRE.FindAllString(commentsJoined(string(b)), -1) {
			bad = append(bad, fmt.Sprintf("%s: %q", path, m))
		}
	}
	if len(bad) > 0 {
		t.Errorf("citations by number; a citation names a heading, numbers move and names stay:\n%s", strings.Join(bad, "\n"))
	}
}

func TestSpecAnchorsFromProse(t *testing.T) {
	spec := "# Part\n\n## Real\n\n```volt\n## Ghost\n```\n\n~~~\n# Phantom\n~~~\n"
	if set := specAnchorsOf(spec); !set["part"] || !set["real"] || set["ghost"] || set["phantom"] {
		t.Errorf("specAnchorsOf = %v; want the headings outside the fences only", set)
	}
}

func githubAnchorOf(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-', r == '_', unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func mdProse(src string) []string {
	lines := strings.Split(src, "\n")
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			lines[i] = ""
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fence = trimmed[:3]
			lines[i] = ""
		}
	}
	return lines
}

func mdAnchors(src string) map[string]bool {
	set := map[string]bool{}
	for _, line := range mdProse(src) {
		if m := mdHeadingRE.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(m[1])
			set[anchorOf(title)] = true
			set[githubAnchorOf(title)] = true
		}
	}
	return set
}

type mdLink struct {
	line         int
	file, anchor string
}

func mdAnchorLinks(src string) []mdLink {
	var out []mdLink
	for i, line := range mdProse(src) {
		for _, m := range mdAnchorLinkRE.FindAllStringSubmatch(line, -1) {
			out = append(out, mdLink{i + 1, m[1], m[2]})
		}
		if m := mdRefDefRE.FindStringSubmatch(line); m != nil {
			out = append(out, mdLink{i + 1, m[1], m[2]})
		}
	}
	return out
}

func TestMarkdownAnchorLinksParsed(t *testing.T) {
	src := "# Real\n\n```md\n# Ghost\n[in code](#ghost)\n```\n\n" +
		"[plain](#real) [titled](#real \"a title\") [other](lint.md#rules)\n\n[ref]: #ghost\n[far]: spec.md#columns \"t\"\n" +
		"[single](#one 'a title') [paren](lint.md#two (a title))\n[sref]: #three 'a title'\n[pref]: spec.md#four (a title)\n"
	if set := mdAnchors(src); !set["real"] || set["ghost"] {
		t.Errorf("mdAnchors = %v; want the heading outside the fence only", set)
	}
	want := []mdLink{{8, "", "real"}, {8, "", "real"}, {8, "lint.md", "rules"}, {10, "", "ghost"}, {11, "spec.md", "columns"},
		{12, "", "one"}, {12, "lint.md", "two"}, {13, "", "three"}, {14, "spec.md", "four"}}
	if got := mdAnchorLinks(src); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("mdAnchorLinks = %v, want %v", got, want)
	}
}

func TestMarkdownAnchorLinksResolve(t *testing.T) {
	docs, err := filepath.Glob(filepath.Join("..", "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	files := append(docs, filepath.Join("..", "README.md"))
	anchors := map[string]map[string]bool{}
	anchorsOf := func(path string) (map[string]bool, bool) {
		path = filepath.Clean(path)
		if set, done := anchors[path]; done {
			return set, set != nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			anchors[path] = nil
			return nil, false
		}
		anchors[path] = mdAnchors(string(b))
		return anchors[path], true
	}
	var bad []string
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range mdAnchorLinks(string(b)) {
			target := path
			if l.file != "" {
				target = filepath.Join(filepath.Dir(path), l.file)
			}
			set, exists := anchorsOf(target)
			switch {
			case !exists:
				bad = append(bad, fmt.Sprintf("%s:%d: %s#%s: no such file", path, l.line, l.file, l.anchor))
			case !set[l.anchor]:
				bad = append(bad, fmt.Sprintf("%s:%d: %s#%s", path, l.line, l.file, l.anchor))
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("markdown links whose anchor names no heading:\n%s", strings.Join(bad, "\n"))
	}
}

func TestDocsIndentWithSpaces(t *testing.T) {
	var bad []string
	for _, path := range proseFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.ContainsRune(line, '\t') {
				bad = append(bad, fmt.Sprintf("%s:%d", path, i+1))
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("a doc indents with four spaces, never a tab, so it reads the same everywhere:\n%s",
			strings.Join(bad, "\n"))
	}
}
