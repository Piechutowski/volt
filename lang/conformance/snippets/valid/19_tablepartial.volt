// spec: §tablepartial, partials with columns, column indexes and an Indexes block injected into a table
table_partial base_template {
  id int [pk]
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
}
table_partial email_index {
  email string [unique]
  email_domain string [index]
  indexes {
    (email_domain, email)
  }
}
table users {
  ~base_template
  ~email_index
  name string
}
