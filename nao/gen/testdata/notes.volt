// every note level, each the note: setting: project, table, column, enum value
Project blog [note: '''
    The blogging platform schema.

    Multi-line notes keep their blank lines and become
    paragraph breaks in godoc.
  '''] {
  database_type: 'PostgreSQL'
}

Enum post_status {
  draft [note: 'Not yet visible to readers']
  published [note: 'Live on the site']
  archived
}

Table posts [note: 'A blog post.'] {
  id int [pk, increment]
  title string [note: 'Shown in lists and in the HTML title tag']
  status post_status
  body string [null, note: '''
    Raw markdown source.
    Rendered to HTML at publish time.
  ''']
  published_at time.Time [null]
}

Table tags {
  id int [pk]
  name string [unique]
}
