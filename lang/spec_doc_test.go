package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	headingRE = regexp.MustCompile(`(?m)^(#{1,3}) (.+)$`)
	tocLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`)

	specCiteRE = regexp.MustCompile(`§([a-z][a-z0-9-]*)`)
	specCodeRE = regexp.MustCompile(`spec/([a-z][a-z0-9-]*)`)
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

func specAnchors(t *testing.T) map[string]bool {
	t.Helper()
	anchors := map[string]bool{}
	for _, m := range headingRE.FindAllStringSubmatch(specText(t), -1) {
		anchors[anchorOf(strings.TrimSpace(m[2]))] = true
	}
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
			case ".git", "node_modules", "target", "grammars":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(d.Name()) {
		case ".go", ".volt", ".dbml", ".md", ".js", ".scm", ".golden", ".sql":
			files = append(files, path)
		}
		return nil
	})
	return files
}

func TestSpecHeadingsUnnumberedAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range headingRE.FindAllStringSubmatch(specText(t), -1) {
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
	for _, m := range headingRE.FindAllStringSubmatch(spec, -1) {
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
