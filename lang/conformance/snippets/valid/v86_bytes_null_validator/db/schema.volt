// spec: §validators, a validator over a []byte column that carries null
// takes rt.Null[[]byte] like every nullable column, and one over a []byte
// column without null takes []byte, which is bytes and never NULL
// (§columns, §nullability)
package db

Table attachments {
  id    int    [pk, increment]
  body  []byte [required]
  thumb []byte [null]

  validators {
    BodySmall(body)
    ThumbSmall(thumb)
  }
}
