// spec: §keywords, §program-structure, project opens no element: a table and a column may be named project
table project {
  id      int    [pk]
  // the name page views are counted under
  project string
}

table page_views {
  id         int [pk]
  project_id int [ref: project.id]
  hits       int [default: 0]
}

records project (id, project) {
  1, "blog"
}
