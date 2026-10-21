// Selects of every shape (spec §selects-over-groups): scalar parameters
// over the model row, a list parameter, literals, a shared projection, a
// star-form exclusion, an ordering and a bare select, so nao_selects.go
// has a golden beside the other generated files.
package metrics

Table page_views {
  id    int [pk, increment]
  site  string
  day   int
  hits  int [default: 0]
  score float64 [default: 0]
  note  string [null]
}

Table link_clicks {
  id     int [pk, increment]
  site   string
  day    int
  target string [default: '']
}

Group traffic {
  page_views
  link_clicks
}

Pred at    { site = :site and day = :day }
Pred since { day >= :from }

Select rows    for page_views where at
Select chosen  for traffic where site in :sites
Select busy    for page_views where hits > 10 and score >= 0.5 and note is not null
Select summary (site, day) for traffic where since
Select public  (* \ target) for link_clicks
Select recent  for traffic where since [order: (day desc, id asc)]
Select all     for link_clicks
