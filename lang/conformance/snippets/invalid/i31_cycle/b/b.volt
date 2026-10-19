package b

import (
	a
)

Table users { id int [pk] }

Scope /b { resources a.users }
