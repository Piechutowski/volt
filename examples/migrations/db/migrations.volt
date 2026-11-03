package db

migration 0001__create_posts {
	create table posts {
		id    int    [pk]
		title string
	}
}

migration 0002__posts_views {
	alter table posts {
		add views int [default: 0]
	}
}
