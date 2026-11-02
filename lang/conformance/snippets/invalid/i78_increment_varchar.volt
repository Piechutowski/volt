// spec: §columns increment extension — increment needs an int column
// want: only an int column can auto-increment
table users {
  code string [pk, increment]
}
