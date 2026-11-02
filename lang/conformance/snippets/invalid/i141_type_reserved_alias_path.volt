// spec: §types, sql is free only with its standard path database/sql
// want: is reserved by generated code for "database/sql"
type sql.X string from "example.com/x"
table t {
  x sql.X
}
