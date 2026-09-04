// spec: §V11.7 — '-' is not a projection operator; exclusion is spelled '\' like group difference
// want: exclusion is spelled '\'
package db

Table page_views {
  id integer [pk]
  site text [not null]
}

Select public (* - site) for page_views
