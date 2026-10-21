// spec: §operators-and-punctuation, '#' is not a token: a setting value spelled with it is refused at the lexer
// want: unexpected character "#"
Table users [headercolor: #3498DB] {
  id int [pk]
}
