// spec: §V12.8 — required is the non-empty rule in both tiers, per type class; enums count as text
package db

Enum plan {
  free
  pro
}

Table accounts {
  id     integer [pk, increment]
  email  varchar [not null, required, unique]
  seats  integer [not null, required]
  tier   plan    [not null, required]
  avatar blob    [not null, required]
  price  decimal(10,2) [not null, required]
}
