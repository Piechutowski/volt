// spec: §selects-over-groups, under group: a limit's order names every group: item, or two groups can tie at the cut
// want: does not name the group: item "site"
package db

table page_views {
  id   int    [pk]
  site string
  hits int
}

select busiest (site, sum(hits) as total) for page_views [group: (site), order: (total desc), limit: 3]
