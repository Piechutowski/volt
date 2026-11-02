// spec: §identifiers, §string-literals, a name is never quoted: a double quote opens a string, and a string is no table name
// want: expected identifier in table name
table "user accounts" {
  id int [pk]
}
