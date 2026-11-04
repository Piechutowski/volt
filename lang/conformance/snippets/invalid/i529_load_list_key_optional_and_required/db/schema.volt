package db

table posts {
  id    int    [pk]
  score int
}

select any_of  for posts where score in :q? [order: (id asc)]
select all_of  for posts where score in :q [order: (id asc)]
