// spec: §migrations, add takes an expression default, NOT NULL or null, without a rebuild
package db

table posts {
  id         int       [pk, increment]
  title      string
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  seen_at    time.Time [null, default: `CURRENT_TIMESTAMP`]
}

migration 0001__init {
  create table posts {
    id    int    [pk, increment]
    title string
  }
}

migration 0002__stamps {
  alter table posts {
    add created_at time.Time [default: `CURRENT_TIMESTAMP`]
    add seen_at    time.Time [null, default: `CURRENT_TIMESTAMP`]
  }
}
