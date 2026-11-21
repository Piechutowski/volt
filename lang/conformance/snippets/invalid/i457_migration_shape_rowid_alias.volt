// spec/migrations, spec/columns, a table_create or table_rebuild shape refuses default: on its rowid alias and autoincrement on a key that carries ref:, and a rebuild that introduces a rowid alias carrying ref: gives it a from:, since NULL would let the database assign ids the referenced table never held
// want: SQLite ignores a DEFAULT on the rowid alias
package db

table users {
  id int [pk]
}

table events {
  id   int    [pk]
  name string
}

table user_profiles {
  user_id int    [pk, ref: users.id]
  bio     string
}

migration 0001__init {
  table_create users {
    id int [pk, default: 1]
  }
  table_create events {
    name string
  }
  table_create user_profiles {
    user_id int    [pk, autoincrement, ref: users.id]
    bio     string
  }
}

migration 0002__keys [irreversible] {
  table_rebuild events {
    id   int    [pk, autoincrement, default: 1]
    name string
  }
  table_rebuild user_profiles {
    owner_id int    [pk, ref: users.id]
    bio      string
  }
}
