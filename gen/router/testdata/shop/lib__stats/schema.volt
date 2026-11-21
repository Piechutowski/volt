// Package db is the shop's metrics, a second package named db, which
// lib__api imports as stats.
package db

table samples {
  id    int    [pk]
  name  string
  value float64
}
