-- name: CreateRecipe :one
INSERT INTO recipes (title, summary, servings, prep_minutes, cook_minutes, source_url, source_kind, notes, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetRecipe :one
SELECT * FROM recipes WHERE id = $1 AND deleted_at IS NULL;

-- name: SearchRecipes :many
SELECT * FROM recipes
WHERE deleted_at IS NULL AND title ILIKE '%' || sqlc.arg(query)::text || '%'
ORDER BY title;

-- name: AddRecipeIngredient :exec
INSERT INTO recipe_ingredients (recipe_id, position, section, quantity, unit, item, prep_note, raw_text)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: RecipeIngredients :many
SELECT * FROM recipe_ingredients WHERE recipe_id = $1 ORDER BY position;

-- name: AddRecipeStep :exec
INSERT INTO recipe_steps (recipe_id, position, text, timer_seconds) VALUES ($1, $2, $3, $4);

-- name: RecipeSteps :many
SELECT * FROM recipe_steps WHERE recipe_id = $1 ORDER BY position;
