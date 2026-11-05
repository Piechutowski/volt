// spec: §selects-over-groups, a table with no key, no primary key and no unique column or index that is never NULL, takes no limit:
// want: order: names no key of hit_log, so two rows can tie and the ones kept at the cut would be arbitrary; a table without a key
package db

table hit_log {
  site string
  hits int
}

select top for hit_log [order: (hits desc, site asc), limit: 5]
