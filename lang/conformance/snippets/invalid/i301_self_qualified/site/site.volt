// spec: §imports, §pipelines, §scopes, §handler-references, §default-handlers, §dataset-declaration, §validators, a package names its own declarations bare: its own name as a qualifier is an error naming the bare form, in each place a declaration is named
// want: qualifies a declaration with this package's own name
package site

table posts {
  id    int    [pk, increment]
  title string

  validators {
    site.TitleShape(title)
  }
}

group pages {
  posts
}

select browse for pages [order: (id asc)]
select picked for posts where id in :ids

pipeline api {
	use site.Session
}

scope /api [pipe: api, error_handler: site.Errors] {
	resources site.posts [default]
	get /picked site.PostPicked
	dataset site.browse
}
