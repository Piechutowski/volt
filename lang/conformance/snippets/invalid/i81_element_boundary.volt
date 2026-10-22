// spec: §line-structure-and-whitespace, an identifier in the first column outside every brace begins a new element; the Ref cannot continue there
// want: found the start of the next element
Table posts {
  id int [pk]
}

Table tags {
  id int [pk]
}

Ref: posts.id >
tags.id
