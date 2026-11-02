// spec: §table-resolution
package app

table posty {
	id int [pk]
}

// "posty" survives English singularization unchanged, so the
// collection and member helpers would both be "Posty".
scope / {
	resources posty
}
