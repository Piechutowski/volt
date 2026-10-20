// spec: §types, the storage type is a built-in, never another declared type
// want: is not a built-in
Type Slug string
Type Handle Slug
Table t {
  h Handle
}
