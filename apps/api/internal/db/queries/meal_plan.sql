-- name: ListMealPlanEntries :many
SELECT
    e.id, e.plan_date, e.slot, e.title, e.recipe_id, e.completed_at, e.completed_by,
    e.notes, e.position, e.created_at, e.updated_at,
    r.title AS recipe_title
FROM meal_plan_entries e
LEFT JOIN recipes r ON r.id = e.recipe_id AND r.deleted_at IS NULL
WHERE e.plan_date >= sqlc.arg(from_date)::date
  AND e.plan_date <= sqlc.arg(to_date)::date
ORDER BY e.plan_date,
    CASE e.slot WHEN 'breakfast' THEN 1 WHEN 'lunch' THEN 2 WHEN 'dinner' THEN 3 ELSE 4 END,
    e.position, e.id;

-- name: GetMealPlanEntry :one
SELECT
    e.id, e.plan_date, e.slot, e.title, e.recipe_id, e.completed_at, e.completed_by,
    e.notes, e.position, e.created_at, e.updated_at,
    r.title AS recipe_title
FROM meal_plan_entries e
LEFT JOIN recipes r ON r.id = e.recipe_id AND r.deleted_at IS NULL
WHERE e.id = $1;

-- name: UpsertMealPlanEntry :one
INSERT INTO meal_plan_entries (plan_date, slot, title, recipe_id, notes)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (plan_date, slot) DO UPDATE SET
    title = EXCLUDED.title,
    recipe_id = EXCLUDED.recipe_id,
    notes = EXCLUDED.notes,
    updated_at = now()
RETURNING *;

-- name: PatchMealPlanEntry :one
UPDATE meal_plan_entries SET
    title = COALESCE(sqlc.narg(title), title),
    notes = COALESCE(sqlc.narg(notes), notes),
    recipe_id = CASE WHEN sqlc.arg(set_recipe)::boolean THEN sqlc.narg(recipe_id)::uuid ELSE recipe_id END,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetMealPlanCompleted :one
UPDATE meal_plan_entries SET
    completed_at = CASE WHEN sqlc.arg(completed)::boolean THEN COALESCE(completed_at, now()) ELSE NULL END,
    completed_by = CASE WHEN sqlc.arg(completed)::boolean THEN COALESCE(completed_by, sqlc.arg(user_id)::uuid) ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteMealPlanEntry :execrows
DELETE FROM meal_plan_entries WHERE id = $1;

-- name: DetachMealPlanRecipe :exec
UPDATE meal_plan_entries SET recipe_id = NULL, updated_at = now() WHERE recipe_id = $1;
