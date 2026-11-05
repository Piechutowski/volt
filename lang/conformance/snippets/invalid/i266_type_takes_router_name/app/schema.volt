// spec: §types, §names, §projects-and-packages, a bare declared type of a package that routes is named apart from the router's names too: Controllers, NewRouter, a controller interface and a reverse-URL helper
// want: takes the Go name of the generated Controllers manifest
package app

type Controllers     string
type NewRouter       string
type PagesController string
type PathHome        string

table posts {
  id    int             [pk]
  a     Controllers
  b     NewRouter
  c     PagesController
  d     PathHome
}

scope / {
  get / Pages.Home
}
