// spec: §settings-lists, ref: appears at most once on a column; tag is the one repeatable setting
// want: setting "ref" may appear at most once
Table users { id int [pk] }
Table admins { id int [pk] }
Table posts {
  owner_id int [ref: users.id, ref: admins.id]
}
