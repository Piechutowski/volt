// Kitchen-sink example: every schema construct of docs/spec.md Part I.
// a comment group a blank line keeps apart
// documents nothing

type Status string

type V2JobStatus string

table_partial base_template {
  id int [pk]
  created_at time.Time [null, index, default: `CURRENT_TIMESTAMP`]
}

// # User accounts
//
// A multi-line **markdown** doc comment.
table users {
  ~base_template
  name string [unique]
  full_name string [null]
  status Status [default: "active"]
  balance float64 [null, default: -100.5]
  two_factor_enabled bool [null, default: false]
  job V2JobStatus [null]
  motto string [null, default: "be kind"]
  ref_col int [null, ref: core_orders.id]

  indexes {
    (id, `lower(name)`) [name: "ix_main", unique]
    `floor(balance)` [name: "ix_balance_floor"]
  }

  validators {
    BalanceFloor(balance)
  }

  records (id, name) {
    1, "ann"
    2, "bob"
  }
}

table core_orders {
  id int [pk]
  user_id int [null]
  total float64 [null]
}

ref: core_orders.user_id > users.id [delete: cascade]

ref: users.(id, name) > core_orders.(id, total)

table empty_target {
  x int [null]
}

records core_orders(id, user_id, total) {
  1, 1, `1.0 + 2.0`
  2, null, 3.5
  4, null, -7
  3, 2, 4.5
}

group commerce {
  users
  core_orders
}

