// Kitchen-sink example: every schema construct of docs/spec.md Part I.
/* block comment
   spanning lines */

Project shop_db [note: """
    # Shop database
    Multi-line **markdown** note.
  """] {
  database_type: "PostgreSQL"
  custom_key: "implementations must accept arbitrary keys"
}

Type Status string

Type V2JobStatus string

TablePartial base_template {
  id int [pk, increment]
  created_at time.Time [null, index, default: `now()`]
}

Table users [note: "user accounts"] {
  ~base_template
  name string [unique]
  full_name string [null]
  status Status [default: "active"]
  balance float64 [null, default: -100.5]
  two_factor_enabled bool [null, default: false]
  job V2JobStatus [null]
  motto string [null, default: "be kind"]
  ref_col int [null, ref: core_orders.id]

  Indexes {
    (id, `lower(name)`) [name: "ix_main", unique]
    `floor(balance)` [name: "ix_balance_floor"]
  }

  Validators {
    BalanceFloor(balance)
  }

  Records (id, name) {
    1, "ann"
    2, "bob"
  }
}

Table core_orders {
  id int [pk]
  user_id int [null]
  total float64 [null]
}

Ref: core_orders.user_id > users.id [delete: cascade]

Ref: users.(id, name) > core_orders.(id, total)

Table empty_target {
  x int [null]
}

Records core_orders(id, user_id, total) {
  1, 1, `1.0 + 2.0`
  2, null, 3.5
  4, null, -7
  3, 2, 4.5
}

Group commerce {
  users
  core_orders
}

