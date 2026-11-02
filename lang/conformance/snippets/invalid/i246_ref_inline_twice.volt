// spec: §settings-lists, ref: appears at most once on a column; tag is the one repeatable setting
// want: setting "ref" may appear at most once
table users { id int [pk] }
table admins { id int [pk] }
table posts {
  owner_id int [ref: users.id, ref: admins.id]
}
