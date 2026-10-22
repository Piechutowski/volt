// analyzers: redundantindex
Table t {
  id int [pk]
  email string [unique]
  country string

  Indexes {
    id [unique] //WANT redundantindex
    email [unique] //WANT redundantindex
    country
    country //WANT redundantindex
    (email, country)
    (email, country) //WANT redundantindex
    (country, id)
  }
}

Table memberships {
  user_id  int
  group_id int

  Indexes {
    (user_id, group_id) [pk, unique] //WANT redundantindex
  }
}

// a where: condition is part of the key: the same columns under
// different conditions are different indexes
Table drafts {
  id int [pk]
  slug string
  deleted_at time.Time [null]

  Indexes {
    slug
    slug [unique, name: "idx_drafts_slug_live", where: `deleted_at IS NULL`]
    slug [name: "idx_drafts_slug_gone", where: `deleted_at IS NOT NULL`]
    slug [name: "idx_drafts_slug_gone_again", where: `deleted_at IS NOT NULL`] //WANT redundantindex
  }
}
