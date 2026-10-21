// spec: §types, §projects-and-packages, a bare Type names a type a Go file of the package declares; one no Go file declares is an error naming the line to write
// want: is declared in no Go file of package db
package db

Type OrderState string
Type Blob []byte

Table orders {
  id     int        [pk, increment]
  status OrderState
  data   Blob
}
