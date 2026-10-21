// spec: §tablepartial — partial with settings, indexes; injection into table
TablePartial base_template [headercolor: #ff0000] {
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
