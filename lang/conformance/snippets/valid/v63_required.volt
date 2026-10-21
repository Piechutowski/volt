// spec: §validators — required is the non-empty rule in both tiers, per type class; enums count as text
package db

Enum plan {
  free
  pro
}

Table accounts {
  id     int [pk, increment]
  email  string [required, unique]
  seats  int [required]
  tier   plan    [required]
  avatar []byte    [required]
  price  float64 [required]
}
