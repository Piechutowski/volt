// spec: §migrations, §the-fold, create table and rebuild table columns take autoincrement, and only a rebuild adds or removes it
package db

table events {
  id   int    [pk, autoincrement]
  name string
}

table points {
  id    int     [pk]
  value float64
}

migration 0001__init {
  create table events {
    id   int    [pk]
    name string
  }
  create table points {
    id    int     [pk, autoincrement]
    value float64
  }
}

migration 0002__ids [irreversible] {
  rebuild table events {
    id   int    [pk, autoincrement]
    name string
  }
  rebuild table points {
    id    int     [pk]
    value float64
  }
}
