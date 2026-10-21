// spec: §enum, enum with a note and quoted values
Enum job_status {
  created [note: 'Waiting to be processed']
  running
  done
  failure
}
Enum grade {
  "A+"
  "A"
  "Not Yet Set"
}
Table jobs {
  status job_status
  grade grade
}
