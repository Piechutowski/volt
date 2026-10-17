// The SQL-to-Go type mapping (docs/spec.md §mapping-to-go). An unmapped type
// is a generation error.
package golang

import (
	"fmt"
	"strings"
)

type goType struct {
	name    string
	imp     string
	nilable bool
}

var typeMap = map[string]goType{
	"tinyint":     {name: "int16"},
	"int2":        {name: "int16"},
	"smallint":    {name: "int16"},
	"smallserial": {name: "int16"},
	"int":         {name: "int32"},
	"integer":     {name: "int32"},
	"int4":        {name: "int32"},
	"mediumint":   {name: "int32"},
	"serial":      {name: "int32"},
	"bigint":      {name: "int64"},
	"int8":        {name: "int64"},
	"bigserial":   {name: "int64"},

	"tinyint unsigned":  {name: "uint8"},
	"smallint unsigned": {name: "uint16"},
	"int unsigned":      {name: "uint32"},
	"integer unsigned":  {name: "uint32"},
	"bigint unsigned":   {name: "uint64"},

	"real":             {name: "float32"},
	"float4":           {name: "float32"},
	"float":            {name: "float64"},
	"float8":           {name: "float64"},
	"double":           {name: "float64"},
	"double precision": {name: "float64"},

	"decimal": {name: "string"},
	"numeric": {name: "string"},
	"money":   {name: "string"},

	"bool":    {name: "bool"},
	"boolean": {name: "bool"},

	"varchar":           {name: "string"},
	"character varying": {name: "string"},
	"char":              {name: "string"},
	"character":         {name: "string"},
	"text":              {name: "string"},
	"tinytext":          {name: "string"},
	"mediumtext":        {name: "string"},
	"longtext":          {name: "string"},
	"citext":            {name: "string"},
	"string":            {name: "string"},
	"uuid":              {name: "string"},

	"date":                        {name: "time.Time", imp: "time"},
	"time":                        {name: "string"},
	"timetz":                      {name: "string"},
	"time with time zone":         {name: "string"},
	"time without time zone":      {name: "string"},
	"timestamp":                   {name: "time.Time", imp: "time"},
	"timestamptz":                 {name: "time.Time", imp: "time"},
	"timestamp with time zone":    {name: "time.Time", imp: "time"},
	"timestamp without time zone": {name: "time.Time", imp: "time"},
	"datetime":                    {name: "time.Time", imp: "time"},

	"json":       {name: "json.RawMessage", imp: "encoding/json", nilable: true},
	"jsonb":      {name: "json.RawMessage", imp: "encoding/json", nilable: true},
	"bytea":      {name: "[]byte", nilable: true},
	"blob":       {name: "[]byte", nilable: true},
	"tinyblob":   {name: "[]byte", nilable: true},
	"mediumblob": {name: "[]byte", nilable: true},
	"longblob":   {name: "[]byte", nilable: true},
	"binary":     {name: "[]byte", nilable: true},
	"varbinary":  {name: "[]byte", nilable: true},
}

func typeResolve(schema, base string, enums map[string]string) (goType, error) {
	key := base
	if schema != "" {
		key = schema + "." + base
	} else {
		key = "public." + base
	}
	if enumType, ok := enums[key]; ok {
		return goType{name: enumType}, nil
	}

	if schema != "" {
		return goType{}, fmt.Errorf("type %s.%s does not name a known enum", schema, base)
	}
	if t, ok := typeMap[strings.ToLower(base)]; ok {
		return t, nil
	}
	return goType{}, fmt.Errorf("no Go mapping for type %q (extend the map in gen/golang/types.go or adjust the schema)", base)
}
