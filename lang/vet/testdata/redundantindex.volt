// analyzers: redundantindex
Table t {
  id int [pk, unique] //WANT redundantindex
  email string [unique]
  country string [index]

  Indexes {
    email [unique, name: "idx_t_email_live", where: `country != ''`] //WANT redundantindex
    (email, country)
    (email, country) [name: "idx_t_email_country_again"] //WANT redundantindex
    (country, id)
  }
}

Table counters {
  id int [pk, index] //WANT redundantindex
  hits int

  Indexes {
    id [unique, name: "idx_counters_live", where: `hits > 0`] //WANT redundantindex
  }
}

// a composite key is unique and indexed: an index on its columns in its
// order repeats it, one on a member alone or in another order does not
Table memberships {
  user_id  int [pk]
  group_id int [pk, index]

  Indexes {
    (user_id, group_id) [unique] //WANT redundantindex
    (group_id, user_id)
  }
}

// a where: condition is part of the key: the same columns under
// different conditions are different indexes
Table drafts {
  id int [pk]
  slug string [index]
  deleted_at time.Time [null]

  Indexes {
    slug [unique, name: "idx_drafts_slug_live", where: `deleted_at IS NULL`]
    slug [name: "idx_drafts_slug_gone", where: `deleted_at IS NOT NULL`]
    slug [name: "idx_drafts_slug_gone_again", where: `deleted_at IS NOT NULL`] //WANT redundantindex
  }
}
