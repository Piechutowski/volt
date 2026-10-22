// spec: §imports, §projects-and-packages, a package's directory path is its package path, imported or not, so each of its segments is an identifier of ASCII letters, digits and underscores, although a package name, like any name, may be Unicode
// want: package directory "żółw" has the segment "żółw", which is not an identifier of ASCII letters, digits and underscores
package żółw

Table users {
  id int [pk, increment]
}
