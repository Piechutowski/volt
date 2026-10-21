// spec: §selects-over-groups, a minted row type does not take the name of a declared type of the package
// want: collides with the declared type Summary
package db

Type Summary string

Table page_views {
  id   int     [pk, increment]
  site string
  note Summary
}

Select summary (site) for page_views
