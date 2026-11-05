// spec: §multi-line-string-literals, a multi-line default with the four escapes, its common indentation trimmed
table posts {
  id int
  body string [default: """
  # Heading
  A quote: "plain", or written \", a single quote ' as itself, and a backslash: \\
    indented once more
  a tab\there
"""]
}
