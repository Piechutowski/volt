// Package inflect derives the singular model name from a plural table
// name: a fixed irregular list, a handful of suffix rules,
// and a confidence bit. When the result is a guess, Singular returns
// ok=false.
package inflect

import "strings"

var irregular = map[string]string{
	"people":   "person",
	"children": "child",
	"men":      "man",
	"women":    "woman",
	"mice":     "mouse",
	"geese":    "goose",
	"feet":     "foot",
	"teeth":    "tooth",
	"indices":  "index",
	"vertices": "vertex",
	"matrices": "matrix",

	"news":    "news",
	"series":  "series",
	"species": "species",
	"data":    "data",
	"media":   "media",

	"statuses":  "status",
	"aliases":   "alias",
	"buses":     "bus",
	"heroes":    "hero",
	"potatoes":  "potato",
	"tomatoes":  "tomato",
	"echoes":    "echo",
	"quizzes":   "quiz",
	"analyses":  "analysis",
	"crises":    "crisis",
	"caches":    "cache",
	"movies":    "movie",
	"cookies":   "cookie",
	"addresses": "address",
	"accesses":  "access",
	"processes": "process",
}

// Singular converts one plural noun to singular. ok reports whether the
// answer is confident; when false the result is a guess.
func Singular(word string) (string, bool) {
	if word == "" {
		return word, false
	}
	lower := strings.ToLower(word)
	if s, ok := irregular[lower]; ok {
		return caseMatch(word, s), true
	}

	switch {
	case strings.HasSuffix(lower, "ies") && len(lower) > 4:
		return word[:len(word)-3] + caseMatch(word[len(word)-3:], "y"), true

	case strings.HasSuffix(lower, "xes"), strings.HasSuffix(lower, "ches"),
		strings.HasSuffix(lower, "shes"), strings.HasSuffix(lower, "zes"),
		strings.HasSuffix(lower, "sses"):
		return word[:len(word)-2], true

	case strings.HasSuffix(lower, "ses"):
		return word[:len(word)-1], true

	case strings.HasSuffix(lower, "ss"), strings.HasSuffix(lower, "us"),
		strings.HasSuffix(lower, "is"):
		return word, false

	case strings.HasSuffix(lower, "s"):
		return word[:len(word)-1], true

	default:
		return word, true
	}
}

// SingularLast inflects only the final underscore-separated segment of a
// compound name: order_items -> order_item, user_roles -> user_role.
func SingularLast(name string) (string, bool) {
	i := strings.LastIndexByte(name, '_')
	if i < 0 || i == len(name)-1 {
		return Singular(name)
	}
	last, ok := Singular(name[i+1:])
	return name[:i+1] + last, ok
}

func caseMatch(src, s string) string {
	if src == strings.ToUpper(src) && src != strings.ToLower(src) {
		return strings.ToUpper(s)
	}
	if len(src) > 0 && src[:1] == strings.ToUpper(src[:1]) && src[:1] != strings.ToLower(src[:1]) {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}
