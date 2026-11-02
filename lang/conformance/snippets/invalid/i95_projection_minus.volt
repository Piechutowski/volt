// spec: §selects-over-groups — '-' is not a projection operator; exclusion is spelled '\' like group difference
// want: exclusion is spelled '\'
package db

table page_views {
  id int [pk]
  site string
}

select public (* - site) for page_views
