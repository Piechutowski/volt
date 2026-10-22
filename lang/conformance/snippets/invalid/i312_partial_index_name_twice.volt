// spec: §indexes, §tablepartial, a partial's own indexes share no name in the tables that inject it: one key twice without name: derives one name, and a column's unique and a partial index on it derive one, reported once at the partial however many tables inject it
// want: declares two indexes named "idx_<table>_created_at_updated_at" in every table that injects it
TablePartial stamps {
  created_at time.Time
  updated_at time.Time
  Indexes {
    (created_at, updated_at)
    (created_at, updated_at)
  }
}

TablePartial contact {
  email string [unique]
  Indexes {
    email [where: `email != ''`]
  }
}

Table posts {
  id int [pk]
  ~stamps
}

Table users {
  id int [pk]
  ~contact
}

Table admins {
  id int [pk]
  ~contact
}
