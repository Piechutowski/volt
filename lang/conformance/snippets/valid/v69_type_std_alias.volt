// spec: §types, a package name generated code imports for itself is free with its standard path
Type sql.NullString string from "database/sql"
Type json.RawMessage []byte from "encoding/json"
Table posts {
  id int [pk]
  label sql.NullString
  body json.RawMessage
}
