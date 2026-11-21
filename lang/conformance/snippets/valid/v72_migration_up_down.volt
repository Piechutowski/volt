// spec/migrations, an up block runs as written and its down block runs as written
package db

table posts {
  id    int    [pk]
  title string
  slug  string
}

migration 0001__create_posts {
  table_create posts {
    id    int    [pk]
    title string
  }
}

migration 0002__posts_slug {
  up {
    table_alter posts {
      add slug string [default: ""]
    }
    exec `UPDATE posts SET slug = lower(title)`
  }
  down {
    table_alter posts {
      drop slug
    }
  }
}
