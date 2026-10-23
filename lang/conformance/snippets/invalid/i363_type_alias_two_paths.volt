// spec: §types, one alias names one package: two Type declarations that bind one alias to two paths are an error
// want: one alias names one package
Type a.Views int from "example.com/one/a"
Type a.Hits  int from "example.com/two/a"

Table metrics {
  id    int     [pk]
  views a.Views
  hits  a.Hits
}
