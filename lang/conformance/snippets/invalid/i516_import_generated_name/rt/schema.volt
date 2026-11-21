package rt

table posts {
  id    int    [pk]
  title string
}

select feed for posts where title = :title? [order: (id asc)]
