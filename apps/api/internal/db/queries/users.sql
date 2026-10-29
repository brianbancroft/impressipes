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

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: SetUserProfile :exec
UPDATE users SET display_name = $2, avatar_color = $3 WHERE id = $1;

-- name: CreateInvite :exec
INSERT INTO invites (token_hash, created_by, member_id, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetInviteForUpdate :one
SELECT * FROM invites WHERE token_hash = $1 FOR UPDATE;

-- name: GetInvite :one
SELECT * FROM invites WHERE token_hash = $1;

-- name: MarkInviteUsed :exec
UPDATE invites SET used_by = $2 WHERE token_hash = $1;

-- name: ReplacePasskeys :exec
DELETE FROM passkeys WHERE user_id = $1;

-- name: AddPasskey :exec
INSERT INTO passkeys (credential_id, user_id, public_key, sign_count, transports)
VALUES ($1, $2, $3, $4, $5);

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = sqlc.arg(email)::text;

-- name: SetUserEmail :exec
UPDATE users SET email = $2 WHERE id = $1;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: CreateLoginCode :exec
INSERT INTO login_codes (user_id, purpose, email, link_hash, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountRecentLoginCodes :one
SELECT count(*) FROM login_codes WHERE email = $1 AND created_at > $2;

-- name: GetLoginCodeByLinkForUpdate :one
SELECT * FROM login_codes WHERE link_hash = $1 FOR UPDATE;

-- name: GetLatestLoginCodeForUpdate :one
SELECT * FROM login_codes
WHERE email = $1 AND used_at IS NULL AND expires_at > now() AND attempts < 5
ORDER BY created_at DESC LIMIT 1
FOR UPDATE;

-- name: BumpLoginCodeAttempts :exec
UPDATE login_codes SET attempts = attempts + 1 WHERE id = $1;

-- name: UseLoginCode :exec
UPDATE login_codes SET used_at = now() WHERE id = $1;
