// spec: §settings-lists, settings are separated by commas: an identifier where a setting ends is a syntax error, not the second word of a setting name
// want: expected ',' or ']' in settings list, found identifier "increment"
Table posts {
  id    int [pk increment]
  title string
}
