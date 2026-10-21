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
