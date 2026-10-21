// spec: §types, §names, §projects-and-packages, a bare declared type of a package that routes is named apart from the router's names too: Controllers, NewRouter, a controller interface and a reverse-URL helper
// want: takes the Go name of the generated Controllers manifest
package app

Type Controllers     string
Type NewRouter       string
Type PagesController string
Type PathHome        string

Table posts {
  id    int             [pk, increment]
  a     Controllers
  b     NewRouter
  c     PagesController
  d     PathHome
}

Scope / {
  get / Pages.Home
}
