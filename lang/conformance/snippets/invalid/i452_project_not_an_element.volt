// spec: §program-structure, project is not an element: a package documents itself with the doc comment above its clause (§doc-comments)
// want: unknown element type "project"
project blog {
  owner: "platform team"
}

table posts {
  id int [pk]
}
