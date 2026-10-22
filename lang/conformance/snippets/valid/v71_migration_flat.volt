// spec: §migrations, a flat body of invertible operations runs backward as their inverses in reverse order
package db

Table users {
  id        int    [pk, increment]
  email     string [unique]
  full_name string
  bio       string [null]

  Indexes {
    full_name
  }
}

Migration 0001__create_users {
  create table users {
    id    int    [pk, increment]
    email string [unique]
    name  string
  }
}

Migration 0002__users_bio {
  alter table users {
    add bio string [null]
    rename name to full_name
  }
  create index users (full_name)
}
