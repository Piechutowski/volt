-- +goose Up
CREATE TABLE post_tags (
    post_id INTEGER NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    tag     TEXT NOT NULL,
    PRIMARY KEY (post_id, tag)
);

CREATE INDEX post_tags_tag ON post_tags (tag, post_id);

-- +goose Down
DROP TABLE post_tags;
