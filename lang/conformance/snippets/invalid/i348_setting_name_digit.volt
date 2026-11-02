// spec: §identifiers, a setting name is an identifier and never begins with a digit
// want: 2fa begins with a digit
Table users {
  id int [pk]
  e  string [2fa]
}
