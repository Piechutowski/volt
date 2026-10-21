// spec: §imports
package a

import (
	b
)

Table users { id int [pk] }

Scope / { resources b.users }
