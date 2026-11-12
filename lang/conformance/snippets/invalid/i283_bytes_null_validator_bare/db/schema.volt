// spec: §validators, a []byte column that carries null is
// rt.Null[rt.Bytes] (§nullability), so a validator taking a bare rt.Bytes
// for it disagrees
// want: column "thumb" ([]byte, Go rt.Null[rt.Bytes]) but parameter 1 of ThumbSmall is rt.Bytes
package db

table attachments {
  id    int    [pk]
  thumb []byte [null]

  validators {
    ThumbSmall(thumb)
  }
}
