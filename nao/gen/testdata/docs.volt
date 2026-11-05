// the doc comments the generators carry (spec §doc-comments-in-generated-code):
// table and column into Go, none into SQL; the index, relationship and
// records docs stay in Volt; the constants of a declared type carry their
// docs in the Go file beside it

type PostState string

// A blog post.
//
// Multi-line doc comments keep their blank lines and become
// paragraph breaks in godoc.
table posts {
  id int [pk]
  // Shown in lists and in the HTML title tag
  title string
  status PostState
  // Raw markdown source.
  // Rendered to HTML at publish time.
  body string [null]
  published_at time.Time [null]
  author_id int // a trailing comment: never carried

  indexes {
    // Lists a writer's posts.
    (author_id, id)
    (published_at, id)
  }

  validators {
    PostStateKnown(status)
  }
}

// a comment the blank line below keeps apart: never carried

table tags {
  id int [pk]
  name string [unique]
}

// A person who writes posts.
table users {
  id int [pk]
}

// The writer of a post.
ref: posts.author_id > users.id

// The tags every blog starts with.
records tags (id, name) {
  1, "news"
}
