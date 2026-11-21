package data

table comments {
  id int [pk]
  post_id string
  author_id string
  ~owned
}
ref: comments.author_id > users.id
ref: comments.post_id > posts.title

table a {
  id int [pk]
  b_id int
}
table b {
  id int [pk]
  a_id int
}
ref: a.b_id > b.id
ref: b.a_id > a.id

table employees {
  id int [pk]
  manager_id int
}
ref: employees.manager_id > employees.id
ref: employees.id > employees.id

table user_limits {
  id int [pk]
}
table user_foos {
  id int [pk]
  bar int
}
table user_foo_bars {
  id int [pk]
}
table bonus {
  id int [pk]
}
table axis {
  id int [pk]
}

table user_columns {
  id int [pk]
}
table user_params [model: "UserParams"] {
  id int [pk]
}
table drafts [model: "UserChangeset"] {
  id int [pk]
}
