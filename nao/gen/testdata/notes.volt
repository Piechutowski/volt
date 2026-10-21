// every note level: project, table (both spellings), column, enum value
Project blog {
  database_type: 'PostgreSQL'
  Note: '''
    The blogging platform schema.

    Multi-line notes keep their blank lines and become
    paragraph breaks in godoc.
  '''
}

Enum post_status {
  draft [note: 'Not yet visible to readers']
  published [note: 'Live on the site']
  archived
}

Table posts [note: 'Setting-style note: loses to the body note below'] {
  id int [pk, increment]
  title string [note: 'Shown in lists and in the HTML title tag']
  status post_status
  body string [null, note: '''
    Raw markdown source.
    Rendered to HTML at publish time.
  ''']
  published_at time.Time [null]

  Note: 'A blog post. The body-form note wins over the setting-form note.'
}

Table tags {
  id int [pk]
  name string [unique]
}
