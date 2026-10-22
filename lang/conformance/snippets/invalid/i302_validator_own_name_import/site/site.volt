// spec: §imports, §validators, where an import takes the package's own name, that name qualifies the import, so a validator written with it names the imported package and is refused as a function of another package, not as a self-qualified one
// want: a validator is a function of the containing package, not of "site": write a local wrapper
package site

import (
	lib/site
)

Table posts {
  id    int    [pk, increment]
  title string

  Validators {
    site.TitleShape(title)
  }
}

Scope /api {
	resources site.tags [default]
}
