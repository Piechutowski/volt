package db

Table users {
  id    integer [pk, increment]
  email text    [not null, unique]
}

Table marks {
  id integer [pk]
}
