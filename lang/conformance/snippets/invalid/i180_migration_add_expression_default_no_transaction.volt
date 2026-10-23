// spec: §migrations, an expression default is added by a schema edit inside the migration's transaction, which a no_transaction migration does not have
// want: a no_transaction migration has none
package db

Table users {
  id      int       [pk]
  created time.Time [default: `CURRENT_TIMESTAMP`]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
}

Migration 0002__created [no_transaction] {
  alter table users {
    add created time.Time [default: `CURRENT_TIMESTAMP`]
  }
}
