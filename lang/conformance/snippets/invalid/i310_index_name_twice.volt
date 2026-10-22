// spec: §indexes, two indexes of one table share no name, derived or written: a column's unique and a partial index on it derive one, and two name: settings write one
// want: has two indexes named "idx_users_email"
Table users {
  id     int    [pk]
  email  string [unique]
  handle string
  region string

  Indexes {
    email [where: `email != ''`]
    (handle, region) [name: "idx_users_lookup"]
    (region, handle) [name: "idx_users_lookup"]
  }
}
