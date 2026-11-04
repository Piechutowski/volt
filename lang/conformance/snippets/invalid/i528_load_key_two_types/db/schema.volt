package db

table posts {
  id    int    [pk]
  title string
  score int
}

select titled for posts where title = :q? [order: (id asc)]
select scored for posts where score in :q? [order: (id asc)]
