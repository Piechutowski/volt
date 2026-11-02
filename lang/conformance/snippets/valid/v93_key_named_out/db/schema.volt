// spec: §query-routes, §generated-client, a key column whose name the client's own locals once took routes like any other
package db

table ms_usage {
  out int    [pk]
  err string
  q   string
}

select by_err for ms_usage where err = :e
