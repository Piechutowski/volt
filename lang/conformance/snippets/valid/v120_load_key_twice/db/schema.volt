package db

table posts {
  id    int    [pk]
  title string
  body  string
}

select two   for posts where title = :a? and body = :b [order: (id asc)]
select pair  for posts where title = :a or body = :b [order: (id asc)]
select by_z  for posts where title = :z [order: (id asc)]
select by_yz for posts where title = :y_z [order: (id asc)]
