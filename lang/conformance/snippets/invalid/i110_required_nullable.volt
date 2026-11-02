// spec: §validators, §columns, required refuses a null column: NULL is absence, not emptiness
// want: a column that carries null has no empty value to refuse
package db

table accounts {
  id    int [pk]
  email string [null, required]
}
