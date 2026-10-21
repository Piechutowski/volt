// the unpleasant cases: quoted names, spaces,
// initialisms, digit-leading names, non-public schemas, keyword-ish names
Enum core.audit_kind {
  "read write" [note: 'Value with a space']
  admin
}
Table users {
  id int [pk]
}
Table core.users [note: 'Same base name as public users; schema prefix disambiguates'] {
  id int [pk]
  api_key string
  homepage_url string [null]
  http_status int [null]
  "full name" string [note: 'Quoted name with a space']
  precision_value float64 [null]
  balance float64 [null]
  kind core.audit_kind
  type string [null, note: 'Column named like a Go keyword']
  func int [null]
}
Table "2fa_codes" {
  code string
  user_id int [ref: > users.id]
  expires_at time.Time
  indexes {
    (user_id, code) [pk]
  }
}
