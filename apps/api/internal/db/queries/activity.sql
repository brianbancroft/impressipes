-- name: RecordActivity :one
INSERT INTO activity (user_id, via, action, list_id, recipe_id, detail, batch_id, undo_of)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: RecentActivity :many
SELECT * FROM activity WHERE list_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2;

-- name: FilterActivity :many
SELECT * FROM activity
WHERE (sqlc.narg(list_id)::uuid IS NULL OR list_id = sqlc.narg(list_id))
  AND (sqlc.narg(user_id)::uuid IS NULL OR user_id = sqlc.narg(user_id))
  AND (sqlc.narg(via)::text IS NULL OR via = sqlc.narg(via) OR (sqlc.narg(via) = 'mcp' AND via LIKE 'mcp:%'))
  AND (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action))
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since))
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetActivity :one
SELECT * FROM activity WHERE id = $1;

-- name: ActivityByBatch :many
SELECT * FROM activity WHERE batch_id = $1 ORDER BY id DESC;

-- name: MarkActivityUndone :execrows
UPDATE activity SET undone_at = now(), undone_by = $2 WHERE id = $1 AND undone_at IS NULL;

-- name: ChangedSince :one
-- True when a later, still-standing change touched the same item or recipe.
SELECT EXISTS (
    SELECT 1 FROM activity
    WHERE id > sqlc.arg(after_id)
      AND undo_of IS NULL
      AND undone_at IS NULL
      AND action NOT IN ('recipe_added_to_list')
      AND ((sqlc.arg(item_id)::text <> '' AND detail->>'itemId' = sqlc.arg(item_id))
        OR (sqlc.narg(recipe_id)::uuid IS NOT NULL AND recipe_id = sqlc.narg(recipe_id)))
)::boolean;
