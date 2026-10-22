// spec: §migrations, §columns, a shape body has one primary key, as a table has
// want: declares a second primary key
package db

Table posts {
  id   int    [pk]
  slug string
}

Migration 0001__init [irreversible] {
  create table posts {
    id   int    [pk]
    slug string [pk]
  }
  rebuild table posts {
    id   int [pk]
    slug string
    Indexes {
      (id, slug) [pk]
    }
  }
}
