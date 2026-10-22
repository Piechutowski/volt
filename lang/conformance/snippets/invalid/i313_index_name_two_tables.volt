// spec: §indexes, §tablepartial, an index name is unique in the schema, since SQLite names indexes per database: a TablePartial whose index carries name: is injected into one table only
// want: tables "users" and "admins" each have an index named "idx_email_ci"
TablePartial contact {
  email string

  Indexes {
    `lower(email)` [unique, name: "idx_email_ci"]
  }
}

Table users {
  id int [pk]
  ~contact
}

Table admins {
  id int [pk]
  ~contact
}
