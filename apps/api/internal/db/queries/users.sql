-- name: CreateUser :one
INSERT INTO users (display_name, role, avatar_color)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at;

-- name: SetUserDisabled :exec
UPDATE users SET disabled_at = CASE WHEN sqlc.arg(disabled)::boolean THEN now() END WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3);

-- name: GetSessionUser :one
SELECT users.* FROM sessions JOIN users ON users.id = sessions.user_id
WHERE sessions.id = $1 AND sessions.expires_at > now();

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;
