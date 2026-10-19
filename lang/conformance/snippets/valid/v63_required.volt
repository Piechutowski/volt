// spec: §validation-checks — required is the non-empty rule in both tiers, per type class; enums count as text
package db

Enum plan {
  free
  pro
}

Table accounts {
  id     int [pk, increment]
  email  string [not null, required, unique]
  seats  int [not null, required]
  tier   plan    [not null, required]
  avatar []byte    [not null, required]
  price  float64 [not null, required]
}
