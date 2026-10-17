// spec: §imports
package a

import (
	b
)

Table users { id integer [pk] }

Scope / { resources b.users }
