package db

table notes {
	id    int    [pk]
	title string [required]
	done  bool   [default: false]
}
