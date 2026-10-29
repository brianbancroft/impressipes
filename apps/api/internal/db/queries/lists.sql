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
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteItem :one
UPDATE list_items SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

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

-- name: GetList :one
SELECT * FROM lists WHERE id = $1 AND NOT archived;

-- name: UpdateList :one
UPDATE lists SET
    name = COALESCE(sqlc.narg(name), name),
    emoji = COALESCE(sqlc.narg(emoji), emoji),
    colour = COALESCE(sqlc.narg(colour), colour),
    sort_order = COALESCE(sqlc.narg(sort_order), sort_order)
WHERE id = sqlc.arg(id) AND NOT archived
RETURNING *;

-- name: ArchiveList :execrows
UPDATE lists SET archived = true WHERE id = $1 AND NOT archived;

-- name: NextListSortOrder :one
SELECT COALESCE(max(sort_order) + 1, 0)::int FROM lists;

-- name: NextCategorySortOrder :one
SELECT COALESCE(max(sort_order) + 1, 0)::int FROM list_categories WHERE list_id = $1;

-- name: GetCategory :one
SELECT * FROM list_categories WHERE id = $1 AND list_id = $2;

-- name: UpdateCategory :one
UPDATE list_categories SET
    name = COALESCE(sqlc.narg(name), name),
    colour = COALESCE(sqlc.narg(colour), colour),
    icon = COALESCE(sqlc.narg(icon), icon),
    sort_order = COALESCE(sqlc.narg(sort_order), sort_order)
WHERE id = sqlc.arg(id) AND list_id = sqlc.arg(list_id)
RETURNING *;

-- name: DeleteCategory :execrows
DELETE FROM list_categories WHERE id = $1 AND list_id = $2;

-- name: GetItem :one
SELECT * FROM list_items WHERE id = $1 AND deleted_at IS NULL;

-- name: InsertItemIfAbsent :one
INSERT INTO list_items (id, list_id, category_id, name, quantity_text, note, added_by, source_recipe_id, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET id = list_items.id
RETURNING *;

-- name: UpdateItem :one
UPDATE list_items SET
    name = COALESCE(sqlc.narg(name), name),
    quantity_text = COALESCE(sqlc.narg(quantity_text), quantity_text),
    note = COALESCE(sqlc.narg(note), note),
    position = COALESCE(sqlc.narg(position), position),
    category_id = CASE WHEN sqlc.arg(set_category)::boolean THEN sqlc.narg(category_id)::uuid ELSE category_id END
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: NextItemPosition :one
SELECT COALESCE(max(position) + 1, 0)::int FROM list_items WHERE list_id = $1 AND deleted_at IS NULL;

-- name: CurrentItemVersion :one
SELECT COALESCE(max(version), 0)::bigint FROM list_items;

-- name: FindOpenItemByName :one
SELECT * FROM list_items
WHERE list_id = $1 AND deleted_at IS NULL AND checked_at IS NULL
  AND lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) = $2
LIMIT 1;

-- name: GetItemMemory :one
SELECT * FROM item_memory WHERE list_id = $1 AND normalized_name = $2;

-- name: GetItemAny :one
SELECT * FROM list_items WHERE id = $1;

-- name: RestoreItem :one
UPDATE list_items SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL
RETURNING *;
