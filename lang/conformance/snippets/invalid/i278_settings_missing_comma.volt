// spec: §settings-lists, settings are separated by commas: two identifiers in a row are a syntax error, not a setting named by two words
// want: expected ',' or ']' in settings list, found identifier "increment"
Table posts {
  id    int [pk increment]
  title string
}
