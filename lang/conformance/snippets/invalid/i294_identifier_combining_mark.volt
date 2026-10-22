// spec: §identifiers, §unicode-general-categories, a combining mark is no letter, so a name written with one is an error
// want: a combining mark, cannot stand in a name
Table places {
  id int [pk]
  café string
}
