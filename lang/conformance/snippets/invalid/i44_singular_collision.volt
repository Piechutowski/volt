// spec: §V5.4
package app

Table posty {
	id integer [pk]
}

// "posty" survives English singularization unchanged, so the
// collection and member helpers would both be "Posty".
Scope / {
	resources posty
}
