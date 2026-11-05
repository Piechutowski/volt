package db

table posts {
  id    int [pk]
  title string
  views int
}

select popular for posts where views >= :min [order: (views desc)]
