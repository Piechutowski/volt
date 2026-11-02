// analyzers: reftarget
table users {
  id int [pk]
  email string
}
table a { x int }
table b { y string }
ref: a.x > users.id
ref: b.y > users.email //WANT reftarget

// composite endpoints: unique is judged against composite indexes
table merchants {
  mid int
  country string
  region string
  indexes {
    (mid, country) [unique]
  }
}
table offices {
  mid int
  country string
  region string
}
ref: offices.(mid, country) > merchants.(mid, country)
ref: offices.(mid, region) > merchants.(mid, region) //WANT reftarget

// a composite primary key is unique as a whole, in any order, and a
// member of it alone is not
table periods {
  merchant_id int [pk]
  day int [pk]
}
table period_notes {
  id int [pk]
  merchant_id int
  day int
}
ref: period_notes.(day, merchant_id) > periods.(day, merchant_id)
ref: period_notes.merchant_id > periods.merchant_id //WANT reftarget

// a unique index with where: constrains only the rows its condition
// admits, and an expression unique index constrains no column, so
// neither is a foreign-key target; a unique column is one
table accounts {
  id int [pk]
  email string
  handle string [unique]
  login string
  deleted_at time.Time [null]
  indexes {
    email [unique, where: `deleted_at IS NULL`]
    (login, `lower(email)`) [unique, name: "idx_accounts_login_email_ci"]
  }
}
table invites {
  id int [pk]
  email string
  handle string
  login string
}
ref: invites.handle > accounts.handle
ref: invites.email > accounts.email //WANT reftarget
ref: invites.login > accounts.login //WANT reftarget

// a one-to-one, unique beside a ref: the declaring column is the
// foreign key and the endpoint it names is the target
table bios {
  id int [pk]
  user_id int [null, unique, ref: users.id]
  user_email string [null, unique, ref: users.email] //WANT reftarget
}
