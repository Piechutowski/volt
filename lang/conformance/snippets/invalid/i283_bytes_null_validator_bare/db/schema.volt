// spec: §validators, a []byte column that carries null is rt.Null[[]byte]
// (§nullability), so a validator taking a bare []byte for it disagrees
// want: column "thumb" ([]byte, Go rt.Null[[]byte]) but parameter 1 of ThumbSmall is []byte
package db

Table attachments {
  id    int    [pk, increment]
  thumb []byte [null]

  validators {
    ThumbSmall(thumb)
  }
}
