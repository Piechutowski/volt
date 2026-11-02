// spec: §keywords, §indexes, §validators, §records-sample-data, a word that opens a block in a table body is lowercase: Indexes, Validators and Records are errors naming the right spelling
// want: write indexes, not Indexes
table users {
  id    int [pk]
  email string

  Indexes {
    (email, id)
  }
  Records (id, email) {
    1, "a@example.com"
  }
}
