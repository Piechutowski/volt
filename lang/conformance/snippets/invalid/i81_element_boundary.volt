// spec: §line-structure-and-whitespace, an identifier in the first column outside every brace begins a new element; the Ref cannot continue there
// want: found the start of the next element
table posts {
  id int [pk]
}

table tags {
  id int [pk]
}

ref: posts.id >
tags.id
