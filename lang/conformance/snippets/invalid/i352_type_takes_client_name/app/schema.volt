// spec: §types, §generated-client, a bare declared type of a package that routes is named apart from the client gen -o writes beside the models: Client and NewClient
// want: takes the Go name of the generated client constructor
package app

Type Client    string
Type NewClient string

Table posts {
  id int       [pk, increment]
  a  Client
  b  NewClient
}

Scope / {
  get / Pages.Home
}
