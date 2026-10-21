// spec: §validators, §columns, required refuses a null column: NULL is absence, not emptiness
// want: "required" and "null" cannot both be set
package db

Table accounts {
  id    int [pk]
  email string [null, required]
}
