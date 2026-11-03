// spec: §columns, autoincrement applies only to the database-assigned
// key, the single-column primary key of the built-in int that is no
// foreign key; SQLite refuses it on a column outside the key
// want: only the database-assigned key, the single-column primary key of the built-in int that is no foreign key, takes AUTOINCREMENT
table users {
  id int [pk]
  counter int [autoincrement]
}
