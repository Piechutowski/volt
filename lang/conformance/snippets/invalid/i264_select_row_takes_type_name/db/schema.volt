// spec: §selects-over-groups, a minted row type does not take the name of a declared type of the package
// want: collides with the declared type Summary
package db

type Summary string

table page_views {
  id   int     [pk]
  site string
  note Summary
}

select summary (site) for page_views
