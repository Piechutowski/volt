// spec: §operators-and-punctuation, ';' is not a token: a line break ends a column
// want: unexpected character ";"
Table users {
  id int [pk];
  name string
}
