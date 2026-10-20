// spec: §migrations, a flat body with a rebuild is accepted when the migration is marked irreversible
package db

Table users {
  id   int [pk, increment]
  hits int [not null, default: 0]
}

Migration 0001__create_users {
  create table users {
    id   int    [pk, increment]
    hits string [not null, default: '0']
  }
}

Migration 0002__int_hits [irreversible, note: 'hits were text'] {
  rebuild table users {
    id   int [pk, increment]
    hits int [not null, default: 0, from: `CAST(hits AS INTEGER)`]
  }
}
