// spec: §string-literals, §multi-line-string-literals, single quotes delimit nothing: a string is written in double quotes and a multi-line string in triple double quotes
// want: a string is written in double quotes
table posts [model: '''
  A multi-line model.
'''] {
  id    int    [pk]
  title string [default: 'a single-quoted default']
}
