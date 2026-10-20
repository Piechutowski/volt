// The migrations fixture: every operation of spec §migrations in both
// body forms, with every migration setting, folding to the schema beside
// it. The Lower golden is blog.sql.golden.
package blog

Table posts {
  id         int       [pk, increment]
  slug       string    [not null, unique]
  title      string    [not null]
  body       string
  views      int       [not null, default: 0]
  published  bool      [not null, default: false]
  created_at time.Time [not null, default: `CURRENT_TIMESTAMP`]

  indexes {
    views [where: `views > 0`]
    (`lower(title)`) [name: 'idx_posts_title_ci']
  }
}

Table comments {
  id      int    [pk, increment]
  post_id int    [not null, ref: > posts.id]
  author  string [not null]
  body    string [not null]
  flagged bool   [not null, default: false]

  indexes {
    (post_id, author)
  }
}

Table post_tags {
  post_id int    [not null, ref: > posts.id]
  tag     string [not null]

  indexes {
    (post_id, tag) [pk]
  }
}

Migration 0001__create_posts {
  create table articles {
    id    int    [pk, increment]
    slug  string [not null, unique]
    title string [not null]
    body  string
    hits  int    [not null, default: 0]

    indexes {
      hits [where: `hits > 0`]
    }
  }
  rename table articles to posts
  alter table posts {
    rename hits to views
    add published bool [not null, default: false]
    add draft string
    drop draft string
  }
  drop index posts views [name: 'idx_articles_hits', where: `views > 0`]
  create index posts views [where: `views > 0`]
  create index posts (`lower(title)`) [name: 'idx_posts_title_ci']
  exec 'UPDATE posts SET views = 0 WHERE views < 0' [down: 'UPDATE posts SET views = 0 WHERE views < 0']
}

Migration 0002__comments_and_tags {
  create table comments {
    id      int    [pk, increment]
    post_id int    [not null, ref: > posts.id]
    author  string [not null]
    body    string [not null]

    indexes {
      (post_id, author)
    }
  }
  create table post_tags {
    post_id int    [not null, ref: > posts.id]
    tag     string [not null]

    indexes {
      (post_id, tag) [pk]
    }
  }
  create table scratch {
    id int [pk]
  }
  drop table scratch {
    id int [pk]
  }
}

Migration 0003__posts_created_at [irreversible, note: 'the timestamp every post should have had'] {
  rebuild table posts {
    id         int       [pk, increment]
    slug       string    [not null, unique]
    title      string    [not null]
    body       string
    views      int       [not null, default: 0, from: `max(views, 0)`]
    published  bool      [not null, default: false]
    created_at time.Time [not null, default: `CURRENT_TIMESTAMP`]

    indexes {
      views [where: `views > 0`]
      (`lower(title)`) [name: 'idx_posts_title_ci']
    }
  }
}

Migration 0004__vacuum [no transaction] {
  up {
    exec 'VACUUM'
  }
}

Migration 0005__comments_flagged {
  up {
    alter table comments {
      add flagged bool [not null, default: false]
    }
  }
  down {
    alter table comments {
      drop flagged
    }
  }
}
