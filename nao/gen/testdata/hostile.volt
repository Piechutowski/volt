// the unpleasant cases: initialisms, an underscore-led name whose
// first word is a digit, names sharing a stem, keyword-ish names,
// a []byte key named like the runtime package the key methods call

type CoreAuditKind string
table users {
  id int [pk]
}
// Shares the stem of users; its own name keeps the models apart
table core_users {
  id int [pk]
  api_key string
  homepage_url string [null]
  http_status int [null]
  // Once a quoted name with a space
  full_name string
  precision_value float64 [null]
  balance float64 [null]
  kind CoreAuditKind
  // Column named like a Go keyword
  type string [null]
  func int [null]
  // Named like the runtime package GetByRt binds through
  rt []byte [unique]

  validators {
    CoreAuditKindKnown(kind)
  }
}
table _2fa_codes {
  code string [pk]
  user_id int [pk, ref: users.id]
  expires_at time.Time
}
table digests {
  // Named like the runtime package Get and Delete bind through
  rt   []byte [pk]
  size int
}
