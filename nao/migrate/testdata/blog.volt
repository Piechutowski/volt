// The migrations fixture: every operation of spec §migrations in both
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
  create table articles {
    id    int    [pk]
    slug  string [unique]
    title string
    body  string [null]
    hits  int    [default: 0]

    indexes {
      hits [where: `hits > 0`]
    }
  }
  rename table articles to posts
  alter table posts {
    rename hits to views
    add published bool [default: false]
    add draft string [null]
    drop draft string [null]
  }
  drop index posts views [name: "idx_articles_hits", where: `views > 0`]
  create index posts views [where: `views > 0`]
  create index posts (`lower(title)`) [name: "idx_posts_title_ci"]
  exec `UPDATE posts SET views = 0 WHERE views < 0` [down: `UPDATE posts SET views = 0 WHERE views < 0`]
}

migration 0002__comments_and_tags {
  create table comments {
    id      int    [pk]
    post_id int    [ref: posts.id]
    author  string
    body    string

    indexes {
      (post_id, author)
    }
  }
  create table post_tags {
    post_id int    [pk, ref: posts.id]
    tag     string [pk]
  }
  create table scratch {
    id int [pk]
  }
  drop table scratch {
    id int [pk]
  }
}

migration 0003__posts_created_at [irreversible] {
  rebuild table posts {
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
    alter table comments {
      add flagged bool [default: false]
    }
  }
  down {
    alter table comments {
      drop flagged
    }
  }
}

migration 0006__nicknames_and_details {
  alter table comments {
    add nickname string [null, unique]
  }
  create table post_details {
    id      int    [pk]
    post_id int    [unique, ref: posts.id]
    summary string [null]
  }
}
