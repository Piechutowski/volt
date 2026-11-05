// spec: §escape-sequences, §groups, a backslash before a line break continues nothing, in a string or between tokens
// want: a backslash before a line break continues nothing
package db

table users {
  id   int    [pk]
  name string [default: """one line \
  continued"""]
}

table admins {
  id int [pk]
}

group people = users \
  admins
