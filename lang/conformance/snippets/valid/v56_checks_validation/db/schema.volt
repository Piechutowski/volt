// spec: §validation-checks — typed checks (SQL CHECK + generated validator), named
// checks, Pred reuse as a parameterless predicate, and Go-reference
// checks (validator only), bare or qualified by the containing package
package db

Pred positive { hits >= 0 }

Table page_views {
  id int [pk]
  site string [not null]
  hits int [not null]
  flag bool [not null, default: false]

  checks {
    positive and hits in (0, 1, 2) [name: 'hits_sane']
    site like '%_' or flag = true
    SiteKnown(site)
    db.SiteKnown(site)
  }
}
