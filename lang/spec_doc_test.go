package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	headingRE  = regexp.MustCompile(`(?m)^(#{1,3}) (.+)$`)
	tocLinkRE  = regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`)
	keyRowRE   = regexp.MustCompile(`(?m)^\| (§V?\d+(?:\.\d+)*|Appendix [A-Z]+(?:\.\d+)?) \| \[([^\]]+)\]`)
	citeRE     = regexp.MustCompile(`§(V?\d+(?:\.\d+)*)`)
	appendixRE = regexp.MustCompile(`Appendix ([A-Z]{1,2})(?:\.(\d+))?`)

	decisionNameRE = regexp.MustCompile(`(?m)^### ([a-z0-9-]+)$`)
	decisionCiteRE = regexp.MustCompile(`\bdecisions? ([a-z][a-z0-9]*(?:-[a-z0-9]+)+(?:, [a-z][a-z0-9]*(?:-[a-z0-9]+)+)*)`)
)

// repoTextFiles lists every source and prose file of the repository
// that may carry a citation.
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

// Every decision is a unique hyphenated heading of docs/decisions.md,
// and every citation of one (`decision name`, `decisions a, b`) names
// such a heading (decision named-sections).
func TestDecisionCitationsResolve(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "docs", "decisions.md"))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, m := range decisionNameRE.FindAllStringSubmatch(string(b), -1) {
		if names[m[1]] {
			t.Errorf("decision %q is declared twice", m[1])
		}
		if !strings.Contains(m[1], "-") {
			t.Errorf("decision %q: a name is hyphenated", m[1])
		}
		names[m[1]] = true
	}
	if len(names) < 50 {
		t.Fatalf("decisions.md looks truncated: %d names", len(names))
	}
	var bad []string
	for _, path := range repoTextFiles(t) {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, m := range decisionCiteRE.FindAllStringSubmatch(string(src), -1) {
			for _, name := range strings.Split(m[1], ", ") {
				if !names[name] {
					bad = append(bad, path+": decision "+name)
				}
			}
		}
	}
	if len(bad) > 0 {
		if len(bad) > 40 {
			bad = append(bad[:40], "…")
		}
		t.Errorf("decision citations that name no heading:\n%s", strings.Join(bad, "\n"))
	}
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

func TestSpecHeadingsUnnumberedAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range headingRE.FindAllStringSubmatch(specText(t), -1) {
		title := strings.TrimSpace(m[2])
		if regexp.MustCompile(`^(§|\d|Appendix [A-Z]+[:.]|[A-C]\.\d)`).MatchString(title) {
			t.Errorf("numbered heading (decision named-sections): %q", title)
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

func TestSpecCitationsResolve(t *testing.T) {
	spec := specText(t)
	key := map[string]bool{}
	for _, m := range keyRowRE.FindAllStringSubmatch(spec, -1) {
		key[m[1]] = true
	}
	if len(key) < 60 {
		t.Fatalf("citation key looks truncated: %d rows", len(key))
	}
	resolves := func(id string) bool {
		parts := strings.Split(id, ".")
		for i := len(parts); i > 0; i-- {
			if key[strings.Join(parts[:i], ".")] {
				return true
			}
		}
		return false
	}
	root := ".."
	var bad []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "target" || name == "grammars" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(name) {
		case ".go", ".volt", ".dbml", ".md", ".js", ".scm":
		default:
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(b)
		for _, m := range citeRE.FindAllStringSubmatch(src, -1) {
			if !resolves("§" + m[1]) {
				bad = append(bad, path+": §"+m[1])
			}
		}
		for _, m := range appendixRE.FindAllStringSubmatch(src, -1) {
			id := "Appendix " + m[1]
			if m[2] != "" {
				id += "." + m[2]
			}
			if !key[id] && !key["Appendix "+m[1]] {
				bad = append(bad, path+": "+id)
			}
		}
		return nil
	})
	if len(bad) > 0 {
		if len(bad) > 40 {
			bad = append(bad[:40], "…")
		}
		t.Errorf("citations that resolve to no heading:\n%s", strings.Join(bad, "\n"))
	}
}
