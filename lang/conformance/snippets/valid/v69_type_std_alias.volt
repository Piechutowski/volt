// spec: §types, a package name generated code imports for itself is free with its standard path
type sql.NullString string from "database/sql"
type json.RawMessage []byte from "encoding/json"
table posts {
  id int [pk]
  label sql.NullString
  body json.RawMessage
}
