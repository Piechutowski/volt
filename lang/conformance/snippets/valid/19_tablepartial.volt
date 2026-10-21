// spec: §tablepartial, partials with columns and indexes injected into a table
TablePartial base_template {
  id int [pk]
  created_at time.Time [default: `now()`]
}
TablePartial email_index {
  email string [unique]
  indexes {
    email [unique]
  }
}
Table users {
  ~base_template
  ~email_index
  name string
}
