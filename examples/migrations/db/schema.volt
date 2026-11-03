package db

table posts {
	id    int    [pk]
	title string [required]
	views int    [default: 0]
}
