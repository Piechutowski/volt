// spec: §migrations, every operation of a migration body, in both forms, with every migration setting
package db

Table accounts {
  id      int    [pk, increment]
  owner   string
  balance int    [default: 0]

  indexes {
    owner [unique]
    balance [where: `balance > 0`]
    (`lower(owner)`) [name: 'idx_accounts_owner_ci']
  }
}

Migration 0001__create_accounts {
  create table ledgers {
    id    int    [pk, increment]
    owner string
    total int    [default: 0]

    indexes {
      owner
    }
  }
  rename table ledgers to accounts
  alter table accounts {
    rename total to balance
    add note string [null]
    drop note string [null]
  }
  drop index accounts owner
  create index accounts owner [unique]
  exec 'INSERT INTO accounts (owner) VALUES (\'root\')' [down: 'DELETE FROM accounts WHERE owner = \'root\'']
}

Migration 0002__accounts_rebuild [irreversible] {
  rebuild table accounts {
    id      int    [pk, increment]
    owner   string
    balance int    [default: 0, from: `balance * 100`]

    indexes {
      owner [unique]
      balance [where: `balance > 0`]
      (`lower(owner)`) [name: 'idx_accounts_owner_ci']
    }
  }
  drop table audit
}

Migration 0003__vacuum [no_transaction] {
  up {
    exec 'VACUUM'
  }
}

Migration 0004__drop_scratch {
  drop table scratch {
    id int [pk]
  }
}
