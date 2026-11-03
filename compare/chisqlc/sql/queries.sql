-- name: GetMember :one
SELECT * FROM members WHERE id = ?;

-- name: CreatePost :one
INSERT INTO posts (slug, title, body, author_id, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPost :one
SELECT * FROM posts WHERE id = ?;

-- name: UpdateDraft :one
UPDATE posts
SET title = coalesce(sqlc.narg('title'), title),
    body = coalesce(sqlc.narg('body'), body)
WHERE id = sqlc.arg('id') AND state = 'draft'
RETURNING *;

-- name: PublishPost :one
UPDATE posts
SET state = 'published', published_at = ?
WHERE id = ? AND state = 'draft'
RETURNING *;

-- name: AddPostTag :exec
INSERT INTO post_tags (post_id, tag) VALUES (?, ?);

-- name: ListPostTags :many
SELECT tag FROM post_tags WHERE post_id = ? ORDER BY tag;

-- name: CreateAudit :exec
INSERT INTO audits (post_id, member_id, action, at) VALUES (?, ?, ?, ?);

-- name: ListAudits :many
SELECT action, member_id, at FROM audits WHERE post_id = ? ORDER BY id;

-- name: ListFeed :many
SELECT p.id, p.slug, p.title, m.name AS author_name, p.published_at, count(c.id) AS comment_count
FROM posts p
JOIN members m ON m.id = p.author_id
LEFT JOIN comments c ON c.post_id = p.id
WHERE p.state = 'published'
  AND (EXISTS (SELECT 1 FROM post_tags t WHERE t.post_id = p.id AND t.tag = sqlc.narg('tag'))
      OR sqlc.narg('tag') IS NULL)
  AND (p.published_at < sqlc.narg('before_at')
      OR (p.published_at = sqlc.narg('before_at') AND p.id < sqlc.narg('before_id'))
      OR sqlc.narg('before_at') IS NULL)
GROUP BY p.id
ORDER BY p.published_at DESC, p.id DESC
LIMIT sqlc.arg('limit');

-- name: CreateComment :one
INSERT INTO comments (post_id, member_id, body, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: CountCommentsSince :one
SELECT count(*) FROM comments
WHERE post_id = ? AND member_id = ? AND created_at > ?;

-- name: ListComments :many
SELECT * FROM comments WHERE post_id = ? ORDER BY created_at, id;

-- name: GetComment :one
SELECT * FROM comments WHERE id = ? AND post_id = ?;

-- name: DeleteComment :exec
DELETE FROM comments WHERE id = ?;

-- name: GetStats :one
SELECT
    (SELECT count(*) FROM posts d WHERE d.author_id = sqlc.arg('member_id') AND d.state = 'draft') AS drafts,
    (SELECT count(*) FROM posts pub WHERE pub.author_id = sqlc.arg('member_id') AND pub.state = 'published') AS published,
    (SELECT count(*) FROM comments c JOIN posts p ON p.id = c.post_id WHERE p.author_id = sqlc.arg('member_id')) AS comments_received;
