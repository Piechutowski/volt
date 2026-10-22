// spec: §escape-sequences, §string-literals, the four escapes of a string, and the characters a string holds as themselves
Table t {
  a string [note: "tab\there"]
  b string [note: "line\nbreak"]
  c string [note: "a \"quoted\" word and a backslash \\"]
  d string [default: "it's plain"]
  e string [tag: "json:\"e,omitempty\""]
}
