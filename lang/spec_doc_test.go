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

	specCiteRE     = regexp.MustCompile(`§[\s\p{Zs}]*([a-z][a-z0-9-]*)`)
	specNumberedRE = regexp.MustCompile(`§[\s\p{Zs}]*[0-9A-Z]|\bAppendix[\s\p{Zs}]+[A-Z]+\b|\bspec/[A-Z]*[0-9]`)
	specRFCRE      = regexp.MustCompile(`RFC[\s\p{Zs}]+[0-9]+(?:` + rfcSep + rfcRef + `)*` + rfcSep + `§?$`)
	specCodeRE     = regexp.MustCompile(`spec/([A-Za-z0-9][A-Za-z0-9-]*)`)
	mdAnchorLinkRE = regexp.MustCompile(`\]\(([A-Za-z0-9_./-]*\.md)?#([^)\s]+)(?:\s+` + mdTitle + `)?\)`)
	mdRefDefRE     = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:\s*([A-Za-z0-9_./-]*\.md)?#(\S+)(?:\s+` + mdTitle + `)?\s*$`)
	mdHeadingRE    = regexp.MustCompile(`^#{1,6} (.+)$`)
)

const (
	rfcSep  = `[\s\p{Zs},;()\x{2013}-]*(?:(?:and|or)[\s\p{Zs}(]*)?`
	rfcRef  = `§{1,2}[\s\p{Zs}]*[0-9][0-9.]*(?:[\s\p{Zs}]*[\x{2013}-][\s\p{Zs}]*[0-9][0-9.]*)?`
	mdTitle = `(?:"[^"]*"|'[^']*'|\([^)]*\))`
)

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

func TestSpecHeadingsUnnumberedAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range specHeadings(specText(t)) {
		title := strings.TrimSpace(m[2])
		if regexp.MustCompile(`^(§|\d|Appendix [A-Z]+[:.]|[A-C]\.\d)`).MatchString(title) {
			t.Errorf("numbered heading: %q", title)
		}
		if len(m[1]) >= 2 && title != "Table of Contents" {
			if seen[title] {
				t.Errorf("duplicate heading %q: anchors would collide", title)
			}
			seen[title] = true
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

// Every spec citation and every diagnostic code
// in the repository names a heading of the spec (decision
// named-sections).
func TestSpecCitationsResolve(t *testing.T) {
	anchors := specAnchors(t)
	var bad []string
	for _, path := range repoTextFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		src := string(b)
		for _, m := range specCiteRE.FindAllStringSubmatch(src, -1) {
			if !anchors[m[1]] {
				bad = append(bad, path+": §"+m[1])
			}
		}
		for _, m := range specCodeRE.FindAllStringSubmatch(src, -1) {
			if !anchors[m[1]] {
				bad = append(bad, path+": spec/"+m[1])
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

func numberedCitations(line string) []string {
	var out []string
	for _, loc := range specNumberedRE.FindAllStringIndex(line, -1) {
		if specRFCRE.MatchString(line[:loc[0]]) {
			continue
		}
		out = append(out, line[loc[0]:loc[1]])
	}
	return out
}

func TestNumberedCitationsCaught(t *testing.T) {
	sect, nbsp, nnbsp := "\u00a7", "\u00a0", "\u202f"
	for line, want := range map[string]string{
		"see " + sect + "6.3":                                                    sect + "6",
		"see " + sect + " 6.3":                                                   sect + " 6",
		"see " + sect + nbsp + "6.3":                                             sect + nbsp + "6",
		"see " + sect + " " + nbsp + " 12":                                       sect + " " + nbsp + " 1",
		"in Append" + "ix B":                                                     "Append" + "ix B",
		"in Append" + "ix VA, collected":                                         "Append" + "ix VA",
		"cites spec/" + "V4":                                                     "spec/" + "V4",
		"cites spec/" + "4":                                                      "spec/" + "4",
		"a date per RFC 3339 " + sect + "5.6":                                    "",
		"a URI per RFC 3986 Append" + "ix A":                                     "",
		"RFC 3339 " + sect + "5.6 but " + sect + "2":                             sect + "2",
		"cites " + sect + "columns and spec/columns":                             "",
		"see the Appendices":                                                     "",
		"see " + sect + "V4.1.4":                                                 sect + "V",
		"see " + sect + "IA, collected":                                          sect + "I",
		"a date per RFC 3339, " + sect + "5.6":                                   "",
		"a date per RFC" + nbsp + "3339 " + sect + "5.6":                         "",
		"a date per RFC 3339" + nbsp + sect + "5.6":                              "",
		"RFC 3339, " + sect + "5.6, then " + sect + "V4":                         sect + "V",
		"per RFC 9110 " + sect + "15.5 and " + sect + "15.6":                     "",
		"per RFC 3339 (" + sect + "5.6)":                                         "",
		"per RFC 3339 " + sect + sect + "5.6-5.7":                                "",
		"see " + sect + nnbsp + "6.3":                                            sect + nnbsp + "6",
		"in Append" + "ix" + nbsp + "VA":                                         "Append" + "ix" + nbsp + "VA",
		"in Append" + "ix" + nnbsp + "B":                                         "Append" + "ix" + nnbsp + "B",
		"per RFC 3339 " + sect + sect + "5.6-5.7 and " + sect + "2":              "",
		"per RFC 9110 (" + sect + "15.5, " + sect + "15.6) or " + sect + "9":     "",
		"per RFC 9110 " + sect + "15.5 or " + sect + "15.6, then " + sect + "V4": sect + "V",
		"per RFC 3339 " + sect + "5.6. See " + sect + "2":                        sect + "2",
	} {
		got := strings.Join(numberedCitations(line), ",")
		if got != want {
			t.Errorf("numberedCitations(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestSpecCitationsNeverNumbered(t *testing.T) {
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
	}
	if len(bad) > 0 {
		t.Errorf("citations by number; a citation names a heading, numbers move and names stay:\n%s", strings.Join(bad, "\n"))
	}
}

func TestSpecCitesParsed(t *testing.T) {
	sect, nbsp, nnbsp := "\u00a7", "\u00a0", "\u202f"
	for line, want := range map[string]string{
		"see " + sect + "columns":                         "columns",
		"see " + sect + " columns":                        "columns",
		"see " + sect + nbsp + "query-routes":             "query-routes",
		"see " + sect + nnbsp + "types, " + sect + "enum": "types,enum",
		"see " + sect + "6.3":                             "",
	} {
		var got []string
		for _, m := range specCiteRE.FindAllStringSubmatch(line, -1) {
			got = append(got, m[1])
		}
		if strings.Join(got, ",") != want {
			t.Errorf("specCiteRE on %q = %q, want %q", line, got, want)
		}
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
