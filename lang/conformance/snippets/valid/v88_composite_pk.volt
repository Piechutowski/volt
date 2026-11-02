// spec: §columns, §migrations, several pk columns form one composite primary key in declaration order, a column a partial injects counted at its injected position, in a table and in a migration's create table and rebuild table alike
package db

table_partial daily {
  day int [pk]
}

table site_days {
  site  string [pk]
  ~daily
  views int    [default: 0]
}

table site_labels {
  site  string [pk]
  label string [unique]
  slot  int    [pk]
}

migration 0001__create_site_days {
  create table site_days {
    site  string [pk]
    day   int    [pk]
    views int    [default: 0]
  }
  create table site_labels {
    site  string [pk]
    label string
    slot  int    [pk]
  }
}

migration 0002__site_labels_unique [irreversible] {
  rebuild table site_labels {
    site  string [pk]
    label string [unique]
    slot  int    [pk]
  }
}
