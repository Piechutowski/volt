// spec: §indexes, §tablepartial, a partial's own indexes share no name in the tables that inject it: one key twice without name: derives one name, and a column's unique and a partial index on it derive one, reported once at the partial however many tables inject it
// want: declares two indexes named "idx_<table>_created_at_updated_at" in every table that injects it
table_partial stamps {
  created_at time.Time
  updated_at time.Time
  indexes {
    (created_at, updated_at)
    (created_at, updated_at)
  }
}

table_partial contact {
  email string [unique]
  indexes {
    email [where: `email != ''`]
  }
}

table posts {
  id int [pk]
  ~stamps
}

table users {
  id int [pk]
  ~contact
}

table admins {
  id int [pk]
  ~contact
}
