// spec: §selects-over-groups — the key with matches on is never NULL
// want: which carries null
package db

table hosts {
  id   int    [pk]
  code string [null, unique]
}

table visits {
  id        int    [pk]
  host_code string [null, ref: hosts.code]
}

select lineup for hosts with visits
