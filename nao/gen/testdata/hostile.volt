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
  api_key string [not null]
  homepage_url string
  http_status int
  "full name" string [not null, note: 'Quoted name with a space']
  precision_value float64
  balance float64
  kind core.audit_kind [not null]
  type string [note: 'Column named like a Go keyword']
  func int
}
Table "2fa_codes" {
  code string [not null]
  user_id int [not null, ref: > users.id]
  expires_at time.Time [not null]
  indexes {
    (user_id, code) [pk]
  }
}
