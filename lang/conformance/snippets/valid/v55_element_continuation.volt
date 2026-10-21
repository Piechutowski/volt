// spec: §line-structure-and-whitespace — an element continues onto further lines when they are indented or inside braces
Table posts [
  note: 'a settings list continues on indented lines'
] {
  id int [pk]
  title string
}

Table tags {
  id int [pk]
}

Ref:
  posts.id > tags.id
