// spec: §escape-sequences, §string-literals, the four escapes of a string, and the characters a string holds as themselves
table t {
  a string [default: "tab\there"]
  b string [default: "line\nbreak"]
  c string [default: "a \"quoted\" word and a backslash \\"]
  d string [default: "it's plain"]
  e string [tag: "json:\"e,omitempty\""]
}
