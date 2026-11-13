// spec: §imports, a declared name that is one of the generated files' own imports needs an alias, and the error names one
// want: import qualifier "rt", which package lib__runtime declares, is the name the generated route files and client import the data runtime nao/rt under (§imports); alias the import: rtdb lib__runtime
package app

import (
	lib__runtime
)

scope / {
	resources rt.posts
}
