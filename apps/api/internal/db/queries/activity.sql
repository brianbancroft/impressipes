-- name: RecordActivity :exec
INSERT INTO activity (user_id, via, action, list_id, recipe_id, detail)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: RecentActivity :many
SELECT * FROM activity WHERE list_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2;
