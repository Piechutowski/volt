// spec: §validators, a validator over a []byte column that carries null
// takes rt.Null[rt.Bytes] like every nullable column, and one over a
// []byte column without null takes rt.Bytes, which is bytes and never
// NULL (§columns, §nullability)
package db

table attachments {
  id    int    [pk]
  body  []byte [required]
  thumb []byte [null]

  validators {
    BodySmall(body)
    ThumbSmall(thumb)
  }
}
