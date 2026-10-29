-- name: CreateList :one
INSERT INTO lists (name, emoji, colour, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListLists :many
SELECT * FROM lists WHERE NOT archived ORDER BY sort_order, created_at;

-- name: CreateListCategory :one
INSERT INTO list_categories (list_id, name, colour, icon, sort_order)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListCategories :many
SELECT * FROM list_categories WHERE list_id = $1 ORDER BY sort_order;

-- name: UpsertListItem :one
INSERT INTO list_items (id, list_id, category_id, name, quantity_text, note, added_by, source_recipe_id, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    quantity_text = EXCLUDED.quantity_text,
    note = EXCLUDED.note,
    position = EXCLUDED.position
RETURNING *;

-- name: SetItemChecked :one
UPDATE list_items
SET checked_at = CASE WHEN sqlc.arg(checked)::boolean THEN now() END,
    checked_by = CASE WHEN sqlc.arg(checked)::boolean THEN sqlc.arg(user_id)::uuid END
WHERE id = $1
RETURNING *;

-- name: SoftDeleteItem :exec
UPDATE list_items SET deleted_at = now() WHERE id = $1;

-- name: ListItems :many
SELECT * FROM list_items WHERE list_id = $1 AND deleted_at IS NULL ORDER BY position;

-- name: ItemsSince :many
SELECT * FROM list_items WHERE version > $1 ORDER BY version;

-- name: RememberItem :exec
INSERT INTO item_memory (list_id, normalized_name, last_category_id, last_quantity)
VALUES ($1, $2, $3, $4)
ON CONFLICT (list_id, normalized_name) DO UPDATE SET
    last_category_id = EXCLUDED.last_category_id,
    last_quantity = EXCLUDED.last_quantity,
    use_count = item_memory.use_count + 1;

-- name: SuggestItems :many
SELECT * FROM item_memory
WHERE list_id = $1 AND normalized_name LIKE sqlc.arg(prefix)::text || '%'
ORDER BY use_count DESC
LIMIT 8;
