// spec: §program-structure, use is not an element: files of one package share a namespace without imports
// want: unknown element type "use"
use * from './base'

Table posts {
  id int [pk]
}
