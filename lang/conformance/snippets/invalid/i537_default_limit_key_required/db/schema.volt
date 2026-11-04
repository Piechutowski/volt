package db

table posts {
  id    int [pk]
  views int
}

select top for posts [order: (id desc), limit: :n]
select by_views for posts where views >= :n [order: (id asc)]
