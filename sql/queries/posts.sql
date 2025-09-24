-- name: CreatePost :exec
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (url) DO NOTHING;

-- name: GetPostsForUser :many
SELECT p.id, p.created_at, p.updated_at, p.title, p.url, p.description, p.published_at, p.feed_id
FROM posts p
JOIN feed_follows ff ON p.feed_id = ff.feed_id
WHERE ff.user_id = $1
ORDER BY COALESCE(p.published_at, p.created_at) DESC
LIMIT $2;

-- name: GetPostsForUserPaginated :many
SELECT p.id, p.created_at, p.updated_at, p.title, p.url, p.description, p.published_at, p.feed_id
FROM posts p
JOIN feed_follows ff ON p.feed_id = ff.feed_id
WHERE ff.user_id = $1
ORDER BY COALESCE(p.published_at, p.created_at) DESC
LIMIT $2 OFFSET $3;

-- name: SearchPosts :many
SELECT p.id, p.created_at, p.updated_at, p.title, p.url, p.description, p.published_at, p.feed_id
FROM posts p
JOIN feed_follows ff ON p.feed_id = ff.feed_id
WHERE ff.user_id = $1 AND (p.title ILIKE $2 OR p.description ILIKE $2)
ORDER BY COALESCE(p.published_at, p.created_at) DESC;