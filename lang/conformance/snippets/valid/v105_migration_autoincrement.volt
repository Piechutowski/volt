// spec/migrations, spec/the-fold, table_create and table_rebuild columns take autoincrement, and only a rebuild adds or removes it
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
  table_create events {
    id   int    [pk]
    name string
  }
  table_create points {
    id    int     [pk, autoincrement]
    value float64
  }
}

migration 0002__ids [irreversible] {
  table_rebuild events {
    id   int    [pk, autoincrement]
    name string
  }
  table_rebuild points {
    id    int     [pk]
    value float64
  }
}
