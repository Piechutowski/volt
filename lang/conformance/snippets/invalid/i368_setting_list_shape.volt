// spec: §settings-lists, a list value is names separated by commas, or every name followed by asc or desc; anything else is no setting value
// want: the list of note: is neither names separated by commas nor names each followed by asc or desc
Table users {
  id int [pk, note: (a asc, b)]
}
