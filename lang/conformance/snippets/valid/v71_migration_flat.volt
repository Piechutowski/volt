// spec: §migrations, a flat body of invertible operations runs backward as their inverses in reverse order
package db

Table users {
  id        int    [pk, increment]
  email     string [not null, unique]
  full_name string [not null]
  bio       string

  indexes {
    full_name
  }
}

Migration 0001__create_users {
  create table users {
    id    int    [pk, increment]
    email string [not null, unique]
    name  string [not null]
  }
}

Migration 0002__users_bio {
  alter table users {
    add bio string
    rename name to full_name
  }
  create index users (full_name)
}
