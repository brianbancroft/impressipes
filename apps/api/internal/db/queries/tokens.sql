-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, name, token_hash) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAPITokenUser :one
SELECT users.*, api_tokens.id AS token_id, api_tokens.name AS token_name
FROM api_tokens JOIN users ON users.id = api_tokens.user_id
WHERE api_tokens.token_hash = $1 AND api_tokens.revoked_at IS NULL;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;

-- name: ListAPITokens :many
SELECT * FROM api_tokens WHERE user_id = $1 AND revoked_at IS NULL ORDER BY created_at;

-- name: RevokeAPIToken :execrows
UPDATE api_tokens SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: RecipeIngredientItems :many
SELECT i.recipe_id, i.item FROM recipe_ingredients i
JOIN recipes r ON r.id = i.recipe_id
WHERE r.deleted_at IS NULL
ORDER BY i.recipe_id, i.position;
