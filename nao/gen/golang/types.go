package golang

import (
	"sort"
	"strconv"
	"strings"
)

func importSplit(key string) (alias, path string) {
	if i := strings.IndexByte(key, '='); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

func importKeysSort(imports map[string]bool) []string {
	keys := make([]string, 0, len(imports))
	for k := range imports {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ai, pi := importSplit(keys[i])
		aj, pj := importSplit(keys[j])
		if pi != pj {
			return pi < pj
		}
		return ai < aj
	})
	return keys
}

func importDecl(key string) string {
	alias, path := importSplit(key)
	if alias != "" {
		return alias + " " + strconv.Quote(path)
	}
	return strconv.Quote(path)
}
