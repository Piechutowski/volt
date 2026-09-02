package b

import (
	a
)

Table users { id integer [pk] }

Scope /b { resources a.users }
