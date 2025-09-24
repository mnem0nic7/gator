-- name: CreatePost :exec
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (url) DO NOTHING;

-- name: GetPostsForUser :many
SELECT 
    posts.id,
    posts.created_at,
    posts.updated_at,
    posts.title,
    posts.url,
    posts.description,
    posts.published_at,
    posts.feed_id
FROM posts
INNER JOIN feeds ON posts.feed_id = feeds.id
INNER JOIN feed_follows ON feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1
ORDER BY COALESCE(posts.published_at, posts.created_at) DESC
LIMIT $2;