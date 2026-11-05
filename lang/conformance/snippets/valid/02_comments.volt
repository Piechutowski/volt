// spec: §comments, §doc-comments, a comment is trivia; the group directly above a construct documents it, and a blank line, a trailing comment and a group above a closing brace document nothing

// a comment to the reader, kept apart by the blank line below

// the table's doc comment,
// two lines of one group
table t {
  // the column's doc comment
  id int // a trailing comment
  //   indented text keeps its spaces after the first

  name string
  // a group above the closing brace
}
