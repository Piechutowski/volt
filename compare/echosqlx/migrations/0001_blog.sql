-- +goose Up
CREATE TABLE members (
    id    INTEGER PRIMARY KEY,
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);

CREATE TABLE posts (
    id           INTEGER PRIMARY KEY,
    slug         TEXT NOT NULL UNIQUE,
    title        TEXT NOT NULL,
    body         TEXT NOT NULL,
    state        TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'published')),
    author_id    INTEGER NOT NULL REFERENCES members (id),
    published_at DATETIME,
    created_at   DATETIME NOT NULL
);

CREATE TABLE comments (
    id         INTEGER PRIMARY KEY,
    post_id    INTEGER NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    member_id  INTEGER NOT NULL REFERENCES members (id),
    body       TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE audits (
    id        INTEGER PRIMARY KEY,
    post_id   INTEGER NOT NULL REFERENCES posts (id),
    member_id INTEGER NOT NULL REFERENCES members (id),
    action    TEXT NOT NULL,
    at        DATETIME NOT NULL
);

INSERT INTO members (id, name, email) VALUES
    (1, 'Ada', 'ada@blog.test'),
    (2, 'Grace', 'grace@blog.test'),
    (3, 'Ken', 'ken@blog.test');

-- +goose Down
DROP TABLE audits;
DROP TABLE comments;
DROP TABLE posts;
DROP TABLE members;
