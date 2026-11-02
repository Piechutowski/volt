// spec: §predicates, a comparison of two columns requires one Go type: an int column does not compare with a float64 column
// want: cannot compare column "hits" (int) with column "score" (float64)
package app

table page_views {
	id    int     [pk]
	hits  int
	score float64
}

select even for page_views where hits = score
