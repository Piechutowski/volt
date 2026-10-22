// spec: §validators — typed checks (SQL CHECK + generated validator), named
// checks, Pred reuse as a parameterless predicate, and Go-reference
// checks (validator only), named bare as every declaration of the package is
package db

Pred positive { hits >= 0 }

Table page_views {
  id int [pk]
  site string
  hits int
  flag bool [default: false]

  Validators {
    SiteKnown(site)
    SiteKnown(site)
  }
}
