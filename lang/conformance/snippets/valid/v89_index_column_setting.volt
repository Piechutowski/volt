// spec/indexes, spec/columns, spec/migrations, a single-column index is the column setting unique or index, in a table, a table_create and an add; index_create and index_drop stay the operations for indexing an existing column
package db

table accounts {
  id     int    [pk]
  email  string [unique]
  handle string [null, unique]
  region string [index]
  note   string [null, index]
  owner  string [index]

  indexes {
    email [unique, name: "idx_accounts_email_eu", where: `region = 'eu'`]
    (region, owner)
  }
}

migration 0001__create_accounts {
  table_create accounts {
    id     int    [pk]
    email  string [unique]
    region string [index]
    owner  string

    indexes {
      email [unique, name: "idx_accounts_email_eu", where: `region = 'eu'`]
      (region, owner)
    }
  }
}

migration 0002__accounts_handle_note {
  table_alter accounts {
    add handle string [null, unique]
    add note string [null, index]
  }
  index_create accounts owner
}
