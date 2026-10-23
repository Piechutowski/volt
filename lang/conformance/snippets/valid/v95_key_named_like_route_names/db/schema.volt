// spec: §generated-query-surface, §route-paths, key columns named like the route files' signature names route by their parameter, which takes a trailing underscore
package db

Table gauges {
  c     int [pk]
  label string
}

Table counters {
  w     string [pk]
  label string
}

Table meters {
  r     int [pk]
  label string
}

Table samples {
  opts  int [pk]
  label string
}

Table spans {
  cs    int [pk]
  label string
}

Table probes {
  volt  int [pk]
  label string
}

Table traces {
  ctx   int [pk]
  label string
}
