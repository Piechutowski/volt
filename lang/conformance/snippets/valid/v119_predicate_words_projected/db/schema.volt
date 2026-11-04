// spec: §keywords, §selects-over-groups, a predicate keyword is an ordinary identifier outside a predicate, so a projection names, aggregates and computes over a column of that name
package db

table flags {
  id   int    [pk]
  like int
  is   int
  site string
}

select listed (id, like, is) for flags [order: (like asc, id asc)]
select summed (site, sum(like) as total, max(is) as top) for flags [group: (site), order: (site asc)]
select bumped (id, like + 1 as next, is * 2 as twice) for flags where site = :site [order: (id asc)]
