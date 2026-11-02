// many-to-many via an explicit junction table with a composite primary key;
// user_id/role_id carry no null: a member of the composite primary key is never NULL
table users {
  id int [pk]
}
table roles {
  id int [pk]
  name string [unique]
}
table user_roles [note: "Junction table: which user holds which role"] {
  user_id int [pk, ref: users.id]
  role_id int [pk, ref: roles.id]
  granted_at time.Time [default: `CURRENT_TIMESTAMP`]
}
