// spec: §string-literals, §multi-line-string-literals, single quotes delimit nothing: a string is written in double quotes and a multi-line string in triple double quotes
// want: a string is written in double quotes
Table posts [note: '''
  A multi-line note.
'''] {
  id    int    [pk]
  title string [note: 'a single-quoted note']
}
