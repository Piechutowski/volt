// spec: §columns, the settings list is the only thing that follows the column type; a bare pk is a syntax error
// want: expected a settings list or end of line after the column type
Table posts {
  id int pk
}
