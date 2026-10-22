// spec: §keywords, §indexes, §validators, §records-sample-data, a word that opens a block in a table body is capitalized: indexes, validators and records are errors naming the right spelling
// want: write Indexes, not indexes
Table users {
  id    int [pk]
  email string

  indexes {
    email
  }
  records (id, email) {
    1, "a@example.com"
  }
}
