// spec: §projects-and-packages, §notes, a package carries no note: its doc comment is written in a hand-written Go file
// want: a package clause takes no settings list
package db [note: "the blog data"]

table posts {
  id    int [pk]
  title string
}
