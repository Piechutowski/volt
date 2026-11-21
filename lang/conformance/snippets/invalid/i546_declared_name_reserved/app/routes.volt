// spec/imports, a declared name that is one of the generated files' own imports needs an
// alias, and the error names one
// want: "rt", which package lib__runtime declares, is the name the generated route files
package app

import (
	lib__runtime
)

scope / {
	resources rt.posts
}
