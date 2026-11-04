// spec: §selects-over-groups, §route-loads, a limit's order is total when it names a key: the primary key, a composite one, a unique column that is never NULL, a unique index over columns alone, the key anywhere in the list, a key of each joined relation of many rows, qualified, and under group: every group: item or a key of the target that group: holds; a load binds the limit from a literal of at least 1, the query string or a path parameter
package app

table authors {
  id   int    [pk]
  name string
}

table posts {
  id        int    [pk]
  slug      string [unique]
  site      string
  day       int
  hits      int
  author_id int    [ref: authors.id]

  indexes {
    (site, day) [unique]
  }
}

table comments {
  id      int [pk]
  post_id int [ref: posts.id]
  votes   int
}

table daily_hits {
  site string [pk]
  day  int    [pk]
  hits int
}

select by_id     for posts [order: (id desc), limit: 10]
select by_slug   for posts [order: (slug asc), limit: 10]
select by_day    for posts [order: (hits desc, site asc, day asc), limit: :limit]
select key_first for posts [order: (id asc, hits desc), limit: 3]
select daily     for daily_hits [order: (hits desc, day desc, site asc), limit: :limit]
select bylines   (id, slug, author.name) for posts join author [order: (author.name asc, id asc), limit: 5]
select voted     (id, slug, comments.votes) for posts join comments [order: (comments.votes desc, id asc, comments.id asc), limit: 5]
select sites     (site, day, sum(hits) as total) for posts [group: (site, day), order: (total desc, day asc, site asc), limit: 5]
select discussed (id, slug, author.name as byline, count(comments.id) as replies) for posts join author join comments [group: (id, slug, byline), order: (replies desc, id asc), limit: :limit]

scope / {
  get /home Pages.Home {
    top   PostByID()
    first PostByDay(5)
    page  PostByDay(count)
  }
  get /days/:n(int) Pages.Days {
    days DailyHitDaily(n)
  }
}
