// spec: §selects-over-groups — '-' is not a projection operator; exclusion is spelled '\' like group difference
// want: exclusion is spelled '\'
package db

Table page_views {
  id int [pk]
  site string [not null]
}

Select public (* - site) for page_views
