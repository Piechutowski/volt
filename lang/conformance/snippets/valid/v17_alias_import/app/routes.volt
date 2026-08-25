package app

import (
	d shared/db
)

Scope / {
	resources posts [model: d.Post]
}
