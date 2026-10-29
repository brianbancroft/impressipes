-- name: CreateRecipe :one
INSERT INTO recipes (title, summary, servings, prep_minutes, cook_minutes, source_url, source_kind, notes, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateRecipe :one
UPDATE recipes SET title = $2, summary = $3, servings = $4, prep_minutes = $5, cook_minutes = $6,
    source_url = $7, source_kind = $8, notes = $9, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteRecipe :execrows
UPDATE recipes SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: GetRecipe :one
SELECT * FROM recipes WHERE id = $1 AND deleted_at IS NULL;

-- name: SearchRecipes :many
SELECT r.* FROM recipes r
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(query)::text = '' OR position(lower(sqlc.arg(query)::text) in lower(r.title)) > 0
       OR position(lower(sqlc.arg(query)::text) in lower(r.summary)) > 0
       OR EXISTS (SELECT 1 FROM recipe_ingredients i WHERE i.recipe_id = r.id
                  AND position(lower(sqlc.arg(query)::text) in lower(i.item)) > 0)
       OR EXISTS (SELECT 1 FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
                  WHERE rt.recipe_id = r.id AND position(lower(sqlc.arg(query)::text) in lower(t.name)) > 0))
  AND (sqlc.arg(tag)::text = '' OR EXISTS (SELECT 1 FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
                                           WHERE rt.recipe_id = r.id AND t.name = sqlc.arg(tag)::text))
  AND (sqlc.arg(mark)::text = '' OR EXISTS (SELECT 1 FROM recipe_favourites f
                                            WHERE f.recipe_id = r.id AND f.user_id = sqlc.arg(user_id)::uuid
                                              AND f.kind = sqlc.arg(mark)::text))
ORDER BY lower(r.title), r.id;

-- name: AddRecipeIngredient :exec
INSERT INTO recipe_ingredients (recipe_id, position, section, quantity, unit, item, prep_note, raw_text)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: RecipeIngredients :many
SELECT * FROM recipe_ingredients WHERE recipe_id = $1 ORDER BY position;

-- name: DeleteRecipeIngredients :exec
DELETE FROM recipe_ingredients WHERE recipe_id = $1;

-- name: AddRecipeStep :exec
INSERT INTO recipe_steps (recipe_id, position, text, timer_seconds) VALUES ($1, $2, $3, $4);

-- name: RecipeSteps :many
SELECT * FROM recipe_steps WHERE recipe_id = $1 ORDER BY position;

-- name: DeleteRecipeSteps :exec
DELETE FROM recipe_steps WHERE recipe_id = $1;

-- name: UpsertTag :one
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: AddRecipeTag :exec
INSERT INTO recipe_tags (recipe_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: DeleteRecipeTags :exec
DELETE FROM recipe_tags WHERE recipe_id = $1;

-- name: TagsForRecipes :many
SELECT rt.recipe_id, t.name FROM recipe_tags rt JOIN tags t ON t.id = rt.tag_id
WHERE rt.recipe_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY t.name;

-- name: ListTags :many
SELECT t.name FROM tags t
WHERE EXISTS (SELECT 1 FROM recipe_tags rt JOIN recipes r ON r.id = rt.recipe_id
              WHERE rt.tag_id = t.id AND r.deleted_at IS NULL)
ORDER BY t.name;

-- name: MarksForRecipes :many
SELECT recipe_id, kind FROM recipe_favourites
WHERE user_id = $1 AND recipe_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY kind;

-- name: MarkRecipe :exec
INSERT INTO recipe_favourites (recipe_id, user_id, kind) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: UnmarkRecipe :exec
DELETE FROM recipe_favourites WHERE recipe_id = $1 AND user_id = $2 AND kind = $3;

-- name: RestoreRecipe :execrows
UPDATE recipes SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: InsertRecipeImage :one
INSERT INTO recipe_images (id, recipe_id, position)
VALUES ($1, $2, (SELECT COALESCE(max(position) + 1, 0) FROM recipe_images WHERE recipe_id = $2))
RETURNING *;

-- name: ListRecipeImages :many
SELECT * FROM recipe_images WHERE recipe_id = $1 ORDER BY position, created_at;

-- name: CountRecipeImages :one
SELECT count(*) FROM recipe_images WHERE recipe_id = $1;

-- name: DeleteRecipeImage :execrows
DELETE FROM recipe_images WHERE id = $1 AND recipe_id = $2;

-- name: GetLiveRecipeImage :one
SELECT i.* FROM recipe_images i
JOIN recipes r ON r.id = i.recipe_id
WHERE i.id = $1 AND r.deleted_at IS NULL;

-- name: RecipeCovers :many
SELECT DISTINCT ON (recipe_id) * FROM recipe_images
WHERE recipe_id = ANY($1::uuid[])
ORDER BY recipe_id, position, created_at;

-- name: LockRecipe :one
SELECT id FROM recipes WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;
