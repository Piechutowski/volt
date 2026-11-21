// spec/migrations, a flat body of invertible operations runs backward as their inverses in reverse order
package db

table users {
  id        int    [pk]
  email     string [unique]
  full_name string [index]
  bio       string [null]
}

migration 0001__create_users {
  table_create users {
    id    int    [pk]
    email string [unique]
    name  string
  }
}

migration 0002__users_bio {
  table_alter users {
    add bio string [null]
    rename name to full_name
  }
  index_create users (full_name)
}
