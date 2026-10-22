// spec: §tablepartial, partials with columns, column indexes and an Indexes block injected into a table
TablePartial base_template {
  id int [pk]
  created_at time.Time [default: `now()`]
}
TablePartial email_index {
  email string [unique]
  email_domain string [index]
  Indexes {
    (email_domain, email)
  }
}
Table users {
  ~base_template
  ~email_index
  name string
}
