// analyzers: refcycle
table a {
  id int [pk]
  b_id int
}
table b { //WANT refcycle
  id int [pk]
  a_id int
}
table c {
  id int [pk]
  a_id int [ref: a.id]
}
ref: a.b_id > b.id
ref: b.a_id > a.id
