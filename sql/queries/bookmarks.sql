-- name: BookmarkPost :exec
INSERT INTO bookmarks (user_id, post_id)
VALUES ($1, $2)
ON CONFLICT (user_id, post_id) DO NOTHING;