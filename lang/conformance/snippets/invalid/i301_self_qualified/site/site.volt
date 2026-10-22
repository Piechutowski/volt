// spec: §imports, §pipelines, §scopes, §handler-references, §default-handlers, §dataset-declaration, §validators, a package names its own declarations bare: its own name as a qualifier is an error naming the bare form, in each place a declaration is named
// want: qualifies a declaration with this package's own name
package site

Table posts {
  id    int    [pk, increment]
  title string

  Validators {
    site.TitleShape(title)
  }
}

Group pages {
  posts
}

Select browse for pages [order: (id asc)]
Select picked for posts where id in :ids

Pipeline api {
	use site.Session
}

Scope /api [pipe: api, error_handler: site.Errors] {
	resources site.posts [default]
	get /picked site.PostPicked
	dataset site.browse
}
