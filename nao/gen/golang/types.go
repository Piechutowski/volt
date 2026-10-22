package golang

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/check"
)

type goType struct {
	name string
	imp  string
}

func typeOf(ct check.ColumnType) goType {
	return goType{name: ct.Go, imp: importKey(ct)}
}

func importKey(ct check.ColumnType) string {
	return importKeyOf(ct.Alias, ct.Import)
}

func importKeyOf(alias, path string) string {
	if path == "" || alias == "" || alias == path[strings.LastIndexByte(path, '/')+1:] {
		return path
	}
	return alias + "=" + path
}

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

// importDecl renders one import line from its key: the path, or
// alias=path for a declared type's package (spec §types).
func importDecl(key string) string {
	alias, path := importSplit(key)
	if alias != "" {
		return alias + " " + strconv.Quote(path)
	}
	return strconv.Quote(path)
}
