// every note level, each the note: setting: project, table, column; the
// constants of a declared type carry their notes in the Go file beside it
Project blog [note: """
    The blogging platform schema.

    Multi-line notes keep their blank lines and become
    paragraph breaks in godoc.
  """] {
  database_type: "SQLite"
}

Type PostState string

Table posts [note: "A blog post."] {
  id int [pk, increment]
  title string [note: "Shown in lists and in the HTML title tag"]
  status PostState
  body string [null, note: """
    Raw markdown source.
    Rendered to HTML at publish time.
  """]
  published_at time.Time [null]

  Validators {
    PostStateKnown(status)
  }
}

Table tags {
  id int [pk]
  name string [unique]
}
