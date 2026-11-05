// spec: §types, a declared type takes no settings list: the doc comment above it documents it, as a Go declaration
type Slug string [model: "Slug"]
table posts {
  id int [pk]
}
