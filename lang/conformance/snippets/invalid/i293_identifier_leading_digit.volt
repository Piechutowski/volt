// spec: §identifiers, a name is a Go identifier and never begins with a digit
// want: begins with a digit
table codes {
  id int [pk]
  2fa_code string
}
