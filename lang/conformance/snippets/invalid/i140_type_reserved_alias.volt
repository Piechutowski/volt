// spec: §types, rt is a package name generated code imports for itself and is never free
// want: is reserved by generated code
type rt.X int from "example.com/x"
table t {
  x rt.X
}
