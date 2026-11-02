// spec: §line-structure-and-whitespace, §imports, recovery inside an import block stops at its closing parenthesis
// want: expected end of line after import specifier
package app

import (
	a b c)
