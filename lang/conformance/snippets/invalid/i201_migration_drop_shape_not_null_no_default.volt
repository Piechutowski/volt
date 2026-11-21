// spec/migrations, the shape of a dropped column is the inverse add and obeys the add rules: a column without null needs a default
// want: whose shape is the inverse add: a column without null is NOT NULL and needs a default:
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id  int    [pk]
    bio string [default: ""]
  }
}

migration 0002__no_bio {
  table_alter users {
    drop bio string
  }
}
