// spec: §selects-over-groups — the key with matches on is never NULL
// want: which carries null
package db

Table hosts {
  id   int    [pk, increment]
  code string [null, unique]
}

Table visits {
  id        int    [pk, increment]
  host_code string [null, ref: hosts.code]
}

Select lineup for hosts with visits
