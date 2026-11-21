// spec/settings-lists, spec/line-structure-and-whitespace, a line break inside the brackets is whitespace: a setting on the next line without a comma is the same syntax error
// want: expected ',' or ']' in settings list, found identifier "update"
table users { id int [pk] }
table posts { user_id int [null] }
ref: posts.user_id > users.id [
  delete: null_set
  update: cascade
]
