// spec: §types, sql is free only with its standard path database/sql
// want: is reserved by generated code for "database/sql"
Type sql.X string from 'example.com/x'
Table t {
  x sql.X
}
