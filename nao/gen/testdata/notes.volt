// every note level, each the note: setting: table, column; the
// constants of a declared type carry their notes in the Go file beside it

type PostState string

table posts [note: """
    A blog post.

    Multi-line notes keep their blank lines and become
    paragraph breaks in godoc.
  """] {
  id int [pk]
  title string [note: "Shown in lists and in the HTML title tag"]
  status PostState
  body string [null, note: """
    Raw markdown source.
    Rendered to HTML at publish time.
  """]
  published_at time.Time [null]

  validators {
    PostStateKnown(status)
  }
}

table tags {
  id int [pk]
  name string [unique]
}
