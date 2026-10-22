// [tag:] (extension) passes struct tags through verbatim: a json
// key replaces the generated default, everything else appends in
// declaration order, and the json key is the wire name a changeset
// casts from (A.5, §changesets).
Table users {
  id        int [pk, increment]
  user_name string [tag: "json:\"userName\"", tag: "xml:\"user-name,attr\""]
  bio       string [null, note: "NULL until written", tag: "msgpack:\"bio,omitempty\""]
}
