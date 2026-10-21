// spec: §predicates: like needs a text column; a declared type is not one, whatever its storage
// want: like needs a text column
package db

Type slug.Slug string from 'example.com/app/slug'

Table posts {
  id   int  [pk, increment]
  slug slug.Slug
}

Select drafts for posts where slug like 'draft-%'
