package data

Table "comments" {
  "id" int [pk]
  post_id varchar
  author_id bigint
  ~owned
}
Ref: comments.author_id > users.id
Ref: comments.post_id > posts.title

Table a {
  id int [pk]
  b_id int
}
Table b {
  id int [pk]
  a_id int
}
Ref: a.b_id > b.id
Ref: b.a_id > a.id

Table employees {
  id int [pk]
  manager_id int
}
Ref: employees.manager_id > employees.id
Ref weird: employees.id - employees.id

Table user_limits {
  id int [pk]
}
Table user_foos {
  id int [pk]
  bar int
}
Table user_foo_bars {
  id int [pk]
}
Table e_users {
  id int [pk]
  kind int
}
Enum user_kind {
  x
}

Table bonus {
  id int [pk]
}
Table axis {
  id int [pk]
}
