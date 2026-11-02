// spec: §line-structure-and-whitespace, §imports, an import block that lost its only entry to a syntax error is not reported empty
// want: expected identifier in import path
package app

import (
	"fmt"
)
