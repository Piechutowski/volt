package golang

import (
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/lang/check"
)

type goType struct {
	name    string
	imp     string
	nilable bool
}

func typeOf(ct check.ColumnType, enumTypes map[string]string) goType {
	if ct.Enum != nil {
		return goType{name: enumTypes[ct.Enum.Key]}
	}
	return goType{name: ct.Go, imp: importKey(ct), nilable: ct.Nilable()}
}

func importKey(ct check.ColumnType) string {
	if ct.Import == "" {
		return ""
	}
	if ct.Alias != "" {
		return ct.Alias + "=" + ct.Import
	}
	return ct.Import
}

// importDecl renders one import line from its key: the path, or
// alias=path for a declared type's package (spec §types).
func importDecl(key string) string {
	if i := strings.IndexByte(key, '='); i >= 0 {
		return key[:i] + " " + strconv.Quote(key[i+1:])
	}
	return strconv.Quote(key)
}
