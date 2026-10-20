// many-to-many via an explicit junction table with a composite primary key;
// user_id/role_id omit 'not null' on purpose: pk-index membership implies it
Table users {
  id int [pk]
}
Table roles {
  id int [pk]
  name string [not null, unique]
}
Table user_roles [note: 'Junction table: which user holds which role'] {
  user_id int [ref: > users.id]
  role_id int [ref: > roles.id]
  granted_at time.Time [not null, default: `now()`]

  indexes {
    (user_id, role_id) [pk]
  }
}
