// spec: §migrations, §columns, a rebuild that introduces a rowid alias carrying ref: gives it a from:, so its values are the referenced rows' and never ids the database assigns
package db

table users {
  id int [pk]
}

table user_profiles {
  user_id int    [pk, ref: users.id]
  bio     string
}

migration 0001__init {
  create table users {
    id int [pk]
  }
  create table user_profiles {
    owner int    [ref: users.id]
    bio   string
  }
}

migration 0002__shared_key [irreversible] {
  rebuild table user_profiles {
    user_id int    [pk, ref: users.id, from: `owner`]
    bio     string
  }
}
