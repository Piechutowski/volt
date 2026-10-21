// spec: §columns increment extension — increment needs an int column
// want: only an int column can auto-increment
Table users {
  code string [pk, increment]
}
