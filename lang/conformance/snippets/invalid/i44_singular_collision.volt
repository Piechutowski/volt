// spec: §table-resolution
package app

Table posty {
	id int [pk]
}

// "posty" survives English singularization unchanged, so the
// collection and member helpers would both be "Posty".
Scope / {
	resources posty
}
