// the unpleasant cases: quoted names, spaces,
// initialisms, digit-leading names, names sharing a stem, keyword-ish names,
// a []byte key named like the runtime package the key methods call
Type CoreAuditKind string
Table users {
  id int [pk]
}
Table core_users [note: 'Shares the stem of users; its own name keeps the models apart'] {
  id int [pk]
  api_key string
  homepage_url string [null]
  http_status int [null]
  "full name" string [note: 'Quoted name with a space']
  precision_value float64 [null]
  balance float64 [null]
  kind CoreAuditKind
  type string [null, note: 'Column named like a Go keyword']
  func int [null]
  rt []byte [unique, note: 'Named like the runtime package GetByRt binds through']

  validators {
    CoreAuditKindKnown(kind)
  }
}
Table "2fa_codes" {
  code string
  user_id int [ref: users.id]
  expires_at time.Time
  indexes {
    (user_id, code) [pk]
  }
}
Table digests {
  rt   []byte [pk, note: 'Named like the runtime package Get and Delete bind through']
  size int
}
