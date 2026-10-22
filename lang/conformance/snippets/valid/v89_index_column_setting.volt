// spec: §indexes, §columns, §migrations, a single-column index is the column setting unique or index, in a table, a create table and an add; create index and drop index stay the operations for indexing an existing column
package db

Table accounts {
  id     int    [pk, increment]
  email  string [unique]
  handle string [null, unique]
  region string [index]
  note   string [null, index]
  owner  string [index]

  Indexes {
    email [unique, name: "idx_accounts_email_eu", where: `region = 'eu'`]
    (region, owner)
  }
}

Migration 0001__create_accounts {
  create table accounts {
    id     int    [pk, increment]
    email  string [unique]
    region string [index]
    owner  string

    Indexes {
      email [unique, name: "idx_accounts_email_eu", where: `region = 'eu'`]
      (region, owner)
    }
  }
}

Migration 0002__accounts_handle_note {
  alter table accounts {
    add handle string [null, unique]
    add note string [null, index]
  }
  create index accounts owner
}
