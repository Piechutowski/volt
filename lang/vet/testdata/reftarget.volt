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
  Indexes {
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

// a composite primary key is unique as a whole, in any order, and a
// member of it alone is not
Table periods {
  merchant_id int [pk]
  day int [pk]
}
Table period_notes {
  id int [pk]
  merchant_id int
  day int
}
Ref: period_notes.(day, merchant_id) > periods.(day, merchant_id)
Ref: period_notes.merchant_id > periods.merchant_id //WANT reftarget

// a unique index with where: constrains only the rows its condition
// admits, and an expression unique index constrains no column, so
// neither is a foreign-key target; a unique column is one
Table accounts {
  id int [pk]
  email string
  handle string [unique]
  login string
  deleted_at time.Time [null]
  Indexes {
    email [unique, where: `deleted_at IS NULL`]
    (login, `lower(email)`) [unique, name: "idx_accounts_login_email_ci"]
  }
}
Table invites {
  id int [pk]
  email string
  handle string
  login string
}
Ref: invites.handle > accounts.handle
Ref: invites.email > accounts.email //WANT reftarget
Ref: invites.login > accounts.login //WANT reftarget

// a one-to-one, unique beside a ref: the declaring column is the
// foreign key and the endpoint it names is the target
Table bios {
  id int [pk]
  user_id int [null, unique, ref: users.id]
  user_email string [null, unique, ref: users.email] //WANT reftarget
}
