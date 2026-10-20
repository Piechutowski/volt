// spec: §types — a bare declared name is the package's own type and takes no from path
// want: takes no from path
Type Slug string from 'example.com/x'
Table t {
  s Slug
}
