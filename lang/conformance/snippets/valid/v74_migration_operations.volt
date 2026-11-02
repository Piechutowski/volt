// spec: §migrations, every operation of a migration body, in both forms, with every migration setting
package db

table accounts {
  id      int    [pk, increment]
  owner   string [unique]
  balance int    [default: 0]

  indexes {
    balance [where: `balance > 0`]
    (`lower(owner)`) [name: "idx_accounts_owner_ci"]
  }
}

migration 0001__create_accounts {
  create table ledgers {
    id    int    [pk, increment]
    owner string [index]
    total int    [default: 0]
  }
  rename table ledgers to accounts
  alter table accounts {
    rename total to balance
    add note string [null]
    drop note string [null]
  }
  drop index accounts owner
  create index accounts owner [unique]
  exec `INSERT INTO accounts (owner) VALUES ('root')` [down: `DELETE FROM accounts WHERE owner = 'root'`]
}

migration 0002__accounts_rebuild [irreversible] {
  rebuild table accounts {
    id      int    [pk, increment]
    owner   string [unique]
    balance int    [default: 0, from: `balance * 100`]

    indexes {
      balance [where: `balance > 0`]
      (`lower(owner)`) [name: "idx_accounts_owner_ci"]
    }
  }
  drop table audit
}

migration 0003__vacuum [no_transaction] {
  up {
    exec `VACUUM`
  }
}

migration 0004__drop_scratch {
  drop table scratch {
    id int [pk]
  }
}
