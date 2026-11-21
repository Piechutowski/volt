// spec/migrations, add takes an expression default, NOT NULL or null, without a rebuild
package db

table posts {
  id         int       [pk]
  title      string
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  seen_at    time.Time [null, default: `CURRENT_TIMESTAMP`]
}

migration 0001__init {
  table_create posts {
    id    int    [pk]
    title string
  }
}

migration 0002__stamps {
  table_alter posts {
    add created_at time.Time [default: `CURRENT_TIMESTAMP`]
    add seen_at    time.Time [null, default: `CURRENT_TIMESTAMP`]
  }
}
