// [tag:] (extension, D60) passes struct tags through verbatim: a json
// key replaces the generated default, everything else appends in
// declaration order, and params structs carry the same tags (A.5).
Table users {
  id        integer [pk, increment]
  user_name varchar [not null, tag: 'json:"userName"', tag: 'xml:"user-name,attr"']
  bio       text [note: 'NULL until written', tag: 'msgpack:"bio,omitempty"']
}
