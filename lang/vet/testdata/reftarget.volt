// analyzers: reftarget
Table users {
  id int [pk]
  email string
}
Table a { x int }
Table b { y string }
Ref: a.x > users.id
Ref: b.y > users.email //WANT reftarget

// composite endpoints: unique is judged against composite indexes
Table merchants {
  mid int
  country string
  region string
  indexes {
    (mid, country) [unique]
  }
}
Table offices {
  mid int
  country string
  region string
}
Ref: offices.(mid, country) > merchants.(mid, country)
Ref: offices.(mid, region) > merchants.(mid, region) //WANT reftarget

// a unique index with where: constrains only the rows its condition
// admits, so it is no foreign-key target
Table accounts {
  id int [pk]
  email string
  handle string
  deleted_at time.Time [null]
  indexes {
    email [unique, where: `deleted_at IS NULL`]
    handle [unique]
  }
}
Table invites {
  id int [pk]
  email string
  handle string
}
Ref: invites.handle > accounts.handle
Ref: invites.email > accounts.email //WANT reftarget

// inline -: the declaring column is the foreign key and the endpoint
// it names is the target
Table bios {
  id int [pk]
  user_id int [null, unique, ref: users.id]
  user_email string [null, unique, ref: users.email] //WANT reftarget
}
