// spec: §projects-and-packages, §doc-comments, a package clause takes no settings list: the doc comment above it documents the package
// want: a package clause takes no settings list
package db [model: "Blog"]

table posts {
  id    int [pk]
  title string
}
