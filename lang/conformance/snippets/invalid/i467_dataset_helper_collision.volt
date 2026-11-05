// spec: §datasets, §route-names-and-reverse-urls, a dataset route whose helper collides with an earlier route's is renamed only by a scope name, which the error names
// want: disambiguate with a scope name
package app

table ms_revenue {
	id int [pk]
}

select rows for ms_revenue

scope /ms {
	get /latest Metrics.Latest [name: ms_revenue_rows]
	dataset rows
}
