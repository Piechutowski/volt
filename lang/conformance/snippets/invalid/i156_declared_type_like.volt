// spec: §predicates: like needs a text column; a declared type is not one, whatever its storage
// want: like needs a text column
package db

type slug.Slug string from "example.com/app/slug"

table posts {
  id   int  [pk]
  slug slug.Slug
}

select drafts for posts where slug like "draft-%"
