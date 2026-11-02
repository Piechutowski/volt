// spec: §generated-query-surface, §route-paths, key columns named like the route files' signature names route by their parameter, which takes a trailing underscore
package db

table gauges {
  c     int [pk]
  label string
}

table counters {
  w     string [pk]
  label string
}

table meters {
  r     int [pk]
  label string
}

table samples {
  opts  int [pk]
  label string
}

table spans {
  cs    int [pk]
  label string
}

table probes {
  volt  int [pk]
  label string
}

table traces {
  ctx   int [pk]
  label string
}
