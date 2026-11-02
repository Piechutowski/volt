// spec: §validators — typed checks (SQL CHECK + generated validator), named
// checks, Pred reuse as a parameterless predicate, and Go-reference
// checks (validator only), named bare as every declaration of the package is
package db

pred positive { hits >= 0 }

table page_views {
  id int [pk]
  site string
  hits int
  flag bool [default: false]

  validators {
    SiteKnown(site)
    SiteKnown(site)
  }
}
