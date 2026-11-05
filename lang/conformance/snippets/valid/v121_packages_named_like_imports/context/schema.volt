package context

table notes {
  id   int    [pk]
  text string
}

scope /context [name: context] {
  get /notes NoteList
}
