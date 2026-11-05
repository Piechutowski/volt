// spec: §line-structure-and-whitespace — an element continues onto further lines when they are indented or inside braces
table posts [
  model: "Article"
] {
  id int [pk]
  title string
}

table tags {
  id int [pk]
}

ref:
  posts.id > tags.id
