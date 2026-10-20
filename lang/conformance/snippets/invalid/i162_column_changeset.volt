// spec: §changesets, a column whose Go field would be Changeset takes the name of the row's changeset method
// want: maps to the Go field Changeset
package db

Table drafts {
  id int [pk]
  changeset string [not null]
}
