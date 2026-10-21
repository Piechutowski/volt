// analyzers: legacyflag
Table t {
  id int pk //WANT legacyflag
  email string unique [not null] //WANT legacyflag
  ok int [pk]
}
