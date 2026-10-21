// spec: §notes, a note is the note: setting; a Note line in a body is not a note
// want: column type
Table users {
  id int [pk]
  Note: 'people'
}
