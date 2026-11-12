// spec: §validators, a []byte column is rt.Bytes in Go, bytes no one
// can change (§columns), so a validator taking a []byte, which a caller
// could change after the check, disagrees
// want: column "body" ([]byte, Go rt.Bytes) but parameter 1 of BodySmall is []byte
package db

table attachments {
  id   int    [pk]
  body []byte

  validators {
    BodySmall(body)
  }
}
