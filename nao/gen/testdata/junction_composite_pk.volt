// many-to-many via an explicit junction table with a composite primary key;
// user_id/role_id carry no null: a member of the pk index is never NULL
Table users {
  id int [pk]
}
Table roles {
  id int [pk]
  name string [unique]
}
Table user_roles [note: "Junction table: which user holds which role"] {
  user_id int [ref: users.id]
  role_id int [ref: roles.id]
  granted_at time.Time [default: `now()`]

  Indexes {
    (user_id, role_id) [pk]
  }
}
