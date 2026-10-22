// spec: §escape-sequences, §groups, a backslash before a line break continues nothing, in a string or between tokens
// want: a backslash before a line break continues nothing
package db

Table users {
  id   int    [pk]
  name string [note: """one line \
  continued"""]
}

Table admins {
  id int [pk]
}

Group people = users \
  admins
