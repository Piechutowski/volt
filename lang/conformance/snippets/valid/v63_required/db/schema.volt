// spec: §validators, required is the non-empty rule in both tiers, per type class; a declared type counts by its storage type
package db

Type Plan string

Table accounts {
  id     int     [pk, increment]
  email  string  [required, unique]
  seats  int     [required]
  tier   Plan    [required]
  avatar []byte  [required]
  price  float64 [required]

  Validators {
    PlanKnown(tier)
  }
}
