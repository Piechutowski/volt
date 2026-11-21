package db

table posts {
  id    int [pk]
  views int
}

select top for posts [order: (views desc, id desc), limit: :limit]
