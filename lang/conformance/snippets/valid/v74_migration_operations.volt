// spec/migrations, every operation of a migration body, in both forms, with every migration setting
package db

table accounts {
  id      int    [pk]
  owner   string [unique]
  balance int    [default: 0]

  indexes {
    balance [where: `balance > 0`]
    (`lower(owner)`) [name: "idx_accounts_owner_ci"]
  }
}

migration 0001__create_accounts {
  table_create ledgers {
    id    int    [pk]
    owner string [index]
    total int    [default: 0]
  }
  table_rename ledgers to accounts
  table_alter accounts {
    rename total to balance
    add note string [null]
    drop note string [null]
  }
  index_drop accounts owner
  index_create accounts owner [unique]
  exec `INSERT INTO accounts (owner) VALUES ('root')` [down: `DELETE FROM accounts WHERE owner = 'root'`]
}

migration 0002__accounts_rebuild [irreversible] {
  table_rebuild accounts {
    id      int    [pk]
    owner   string [unique]
    balance int    [default: 0, from: `balance * 100`]

    indexes {
      balance [where: `balance > 0`]
      (`lower(owner)`) [name: "idx_accounts_owner_ci"]
    }
  }
  table_drop audit
}

migration 0003__vacuum [no_transaction] {
  up {
    exec `VACUUM`
  }
}

migration 0004__drop_scratch {
  table_drop scratch {
    id int [pk]
  }
}
