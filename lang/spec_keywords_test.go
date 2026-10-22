package lang

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/parser"
)

var keywordRowRE = regexp.MustCompile("^\\| ([a-z ]+) \\| (.+) \\| §")

// The §keywords table lists every keyword in its one spelling, and the
// parser's own lists, which the language server completes from, agree
// with it class by class.
func TestSpecKeywordsTableMatchesTheParser(t *testing.T) {
	spec := specText(t)
	start := strings.Index(spec, "\n### Keywords\n")
	end := strings.Index(spec, "\n### String Literals\n")
	if start < 0 || end < start {
		t.Fatal("no §keywords section")
	}
	classes := map[string][]string{}
	for _, line := range strings.Split(spec[start:end], "\n") {
		m := keywordRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var words []string
		for _, w := range strings.Split(m[2], ",") {
			words = append(words, strings.Trim(strings.TrimSpace(w), "`"))
		}
		classes[m[1]] = words
	}
	same := func(class string, want []string) {
		t.Helper()
		got := append([]string(nil), classes[class]...)
		want = append([]string(nil), want...)
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("§keywords lists the %s as %v; the parser has %v", class, got, want)
		}
	}
	var elements []string
	for _, w := range parser.ElementWords {
		if w != "package" && w != "import" {
			elements = append(elements, w)
		}
	}
	same("element and block words", append(elements, "Indexes", "Validators"))
	same("package words", []string{"package", "import"})
	same("setting names", parser.SettingNames)
	same("referential actions", parser.RefActions)
	same("resource actions", parser.ResourceActions)
	same("parameter types", parser.ParamTypes)
	for _, w := range parser.ParamTypes {
		if !KnownParamType(w) {
			t.Errorf("parameter types: %s is in the parser's list but not a parameter type of the checker", w)
		}
	}
	same("route words", append(append([]string(nil), parser.Verbs...), "resources", "dataset", "use"))
	for class, words := range classes {
		for _, w := range words {
			lower := strings.ToLower(w)
			if class != "element and block words" && w != lower {
				t.Errorf("%s: %s is not lowercase, and only the element and block words are capitalized", class, w)
			}
			if class == "element and block words" && w == lower {
				t.Errorf("%s: %s is not capitalized", class, w)
			}
		}
	}
}
