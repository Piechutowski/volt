package db

table posts {
  id    int    [pk]
  title string
}

select titled for posts where title = :title [order: (id asc)]
