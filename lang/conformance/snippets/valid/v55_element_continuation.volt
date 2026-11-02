// spec: §line-structure-and-whitespace — an element continues onto further lines when they are indented or inside braces
table posts [
  note: "a settings list continues on indented lines"
] {
  id int [pk]
  title string
}

table tags {
  id int [pk]
}

ref:
  posts.id > tags.id
