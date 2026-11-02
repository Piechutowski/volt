// spec: §project, §projects-and-packages, a package holds at most one Project, whichever files declare them
// want: at most one Project element per package
package db

Project blog {
}

Table posts {
  id int [pk]
}
