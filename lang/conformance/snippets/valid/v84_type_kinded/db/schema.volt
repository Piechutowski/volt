// spec: §types, §predicates, §validators: a value set is a declared Type whose Go declaration spells its storage type; its constants and its membership validator are Go beside the schema, its defaults and records are literals of the storage type, and its columns compare with = and in
package db

type OrderState string
type Priority int

table orders {
  id       int        [pk, increment]
  status   OrderState [default: "pending"]
  prev     OrderState [null]
  priority Priority   [default: 2]

  validators {
    OrderStateKnown(status)
    OrderStateKnownNull(prev)
  }

  records (id, status, priority) {
    1, "paid", 3
  }
}

select by_state  for orders where status in :states
select urgent    for orders where priority = 3 and status != :skip
select unplaced  for orders where prev is null
