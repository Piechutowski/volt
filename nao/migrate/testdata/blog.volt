// The migrations fixture: every operation of spec/migrations in both
// body forms, with every migration setting, folding to the schema beside
// it. The Lower golden is blog.sql.golden.

package blog

table posts {
  id         int       [pk]
  slug       string    [unique]
  title      string
  body       string    [null]
  views      int       [default: 0]
  published  bool      [default: false]
  created_at time.Time [default: `CURRENT_TIMESTAMP`]

  indexes {
    views [where: `views > 0`]
    (`lower(title)`) [name: "idx_posts_title_ci"]
  }
}

table comments {
  id       int    [pk]
  post_id  int    [ref: posts.id]
  author   string
  body     string
  flagged  bool   [default: false]
  nickname string [null, unique]

  indexes {
    (post_id, author)
  }
}

table post_tags {
  post_id int    [pk, ref: posts.id]
  tag     string [pk]
}

table post_details {
  id      int    [pk]
  post_id int    [unique, ref: posts.id]
  summary string [null]
}

migration 0001__create_posts {
  table_create articles {
    id    int    [pk]
    slug  string [unique]
    title string
    body  string [null]
    hits  int    [default: 0]

    indexes {
      hits [where: `hits > 0`]
    }
  }
  table_rename articles to posts
  table_alter posts {
    rename hits to views
    add published bool [default: false]
    add draft string [null]
    drop draft string [null]
  }
  index_drop posts views [name: "idx_articles_hits", where: `views > 0`]
  index_create posts views [where: `views > 0`]
  index_create posts (`lower(title)`) [name: "idx_posts_title_ci"]
  exec `UPDATE posts SET views = 0 WHERE views < 0` [down: `UPDATE posts SET views = 0 WHERE views < 0`]
}

migration 0002__comments_and_tags {
  table_create comments {
    id      int    [pk]
    post_id int    [ref: posts.id]
    author  string
    body    string

    indexes {
      (post_id, author)
    }
  }
  table_create post_tags {
    post_id int    [pk, ref: posts.id]
    tag     string [pk]
  }
  table_create scratch {
    id int [pk]
  }
  table_drop scratch {
    id int [pk]
  }
}

migration 0003__posts_created_at [irreversible] {
  table_rebuild posts {
    id         int       [pk]
    slug       string    [unique]
    title      string
    body       string    [null]
    views      int       [default: 0, from: `max(views, 0)`]
    published  bool      [default: false]
    created_at time.Time [default: `CURRENT_TIMESTAMP`]

    indexes {
      views [where: `views > 0`]
      (`lower(title)`) [name: "idx_posts_title_ci"]
    }
  }
}

migration 0004__vacuum [no_transaction] {
  up {
    exec `VACUUM`
  }
}

migration 0005__comments_flagged {
  up {
    table_alter comments {
      add flagged bool [default: false]
    }
  }
  down {
    table_alter comments {
      drop flagged
    }
  }
}

migration 0006__nicknames_and_details {
  table_alter comments {
    add nickname string [null, unique]
  }
  table_create post_details {
    id      int    [pk]
    post_id int    [unique, ref: posts.id]
    summary string [null]
  }
}
