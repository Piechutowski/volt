// spec: §project, §projects-and-packages, a package holds at most one Project, whichever files declare them
// want: at most one project element per package
package db

project blog {
}

table posts {
  id int [pk]
}
