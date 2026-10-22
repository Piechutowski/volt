package db

Table posts {
  id    int [pk, increment]
  title string
  views int
}

Select popular for posts where views >= :min [order: (views desc)]
