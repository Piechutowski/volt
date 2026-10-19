// spec: §columns increment extension — increment applies only to the
// single-column primary key; elsewhere it would be silently ignored
// want: only the primary key auto-increments
Table users {
  id int [pk]
  counter int [not null, increment]
}
