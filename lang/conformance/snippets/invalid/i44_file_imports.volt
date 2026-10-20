// spec: §file-imports-removed — file imports are not part of Volt; every form is rejected
use * from './base'
use {
  table auth.users as u
  table auth.roles as r
  enum job_status
  tablepartial base_template
  note reminder
  schema auth
  tablegroup auth_core
} from './auth.volt'
reuse * from './common/index'
