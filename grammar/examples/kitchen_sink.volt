// Kitchen-sink example: every schema construct of docs/spec.md Part I.
/* block comment
   spanning lines */

Project shop_db {
  database_type: 'PostgreSQL'
  custom_key: 'implementations must accept arbitrary keys'
  Note: '''
    # Shop database
    Multi-line **markdown** note.
  '''
}

Enum status {
  active [note: 'operational']
  retired
  "Not Yet Set"
}

Enum v2.job_status {
  queued
  running
}

TablePartial base_template [headercolor: #ff0000] {
  id int [pk, increment]
  created_at time.Time [null, default: `now()`]
}

Table users as U [headercolor: #3498DB, note: 'main table'] {
  ~base_template
  name string [unique]
  full_name "double precision" [null]
  status status [default: status.active]
  balance float64 [null, default: -100.5]
  2fa_enabled bool [null, default: false]
  job v2.job_status [null, default: null]
  motto string [null, default: 'be kind']
  ref_col int [null, ref: > core.orders.id, ref: - U.name]

  Note: 'user accounts'

  indexes {
    (id, `lower(name)`) [name: 'ix_main', unique]
    created_at [type: btree, note: 'time queries']
    `floor(balance)` [name: 'ix_balance_floor']
  }

  validators {
    BalanceFloor(balance)
  }

  Records (id, name) {
    1, 'ann'
    2, 'bob'
  }
}

Table core.orders {
  id int [pk]
  user_id int [null]
  total float64 [null]
}

Ref: U.id < core.orders.user_id [delete: cascade, update: no action]

Ref fk_composite {
  users.(id, name) > core.orders.(id, total) [color: #00ff00]
}

Ref: users.id <> core.orders.id

Table empty_target {
  x int [null]
}

Records core.orders(id, user_id, total) {
  1, 1, `1.0 + 2.0`
  2, , 3.5
  , null, -7
  3, 2, 'true'
}

TableGroup commerce [color: #cccccc, note: 'sales tables'] {
  users
  core.orders
  Note: 'group note'
}

Note sticky_reminder [color: none] {
  '''remember to *normalize*'''
}

DiagramView overview {
  Tables {
    users
    core.orders
  }
  TableGroups { * }
  Schemas {
  }
}
