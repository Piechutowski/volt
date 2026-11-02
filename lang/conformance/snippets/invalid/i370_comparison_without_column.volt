// spec: §predicates, a comparison names a column on at least one side
// want: a comparison needs a column on one side
package app

table page_views {
	id   int [pk]
	hits int
}

select params for page_views where :a = :b
select literals for page_views where 1 = 1
