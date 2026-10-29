-- name: CreateOAuthClient :one
INSERT INTO oauth_clients (id, name, redirect_uris) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- name: CreateOAuthCode :exec
INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ConsumeOAuthCode :one
UPDATE oauth_codes SET used_at = now()
WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING *;

-- name: CreateOAuthGrant :one
INSERT INTO oauth_grants (client_id, user_id, token_hash, expires_at, refresh_hash, refresh_expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOAuthGrantByAccess :one
SELECT users.*, oauth_grants.id AS grant_id, oauth_clients.name AS client_name
FROM oauth_grants
JOIN users ON users.id = oauth_grants.user_id
JOIN oauth_clients ON oauth_clients.id = oauth_grants.client_id
WHERE oauth_grants.token_hash = $1 AND oauth_grants.revoked_at IS NULL AND oauth_grants.expires_at > now();

-- name: GetOAuthGrantByRefreshForUpdate :one
SELECT * FROM oauth_grants
WHERE refresh_hash = $1 AND revoked_at IS NULL AND refresh_expires_at > now()
FOR UPDATE;

-- name: RotateOAuthGrant :exec
UPDATE oauth_grants
SET token_hash = $2, expires_at = $3, refresh_hash = $4, refresh_expires_at = $5
WHERE id = $1;

-- name: TouchOAuthGrant :exec
UPDATE oauth_grants SET last_used_at = now() WHERE id = $1;

-- name: ListOAuthGrants :many
SELECT oauth_grants.id, oauth_grants.created_at, oauth_grants.last_used_at, oauth_clients.name AS client_name
FROM oauth_grants JOIN oauth_clients ON oauth_clients.id = oauth_grants.client_id
WHERE oauth_grants.user_id = $1 AND oauth_grants.revoked_at IS NULL
ORDER BY oauth_grants.created_at;

-- name: RevokeOAuthGrant :execrows
UPDATE oauth_grants SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: RevokeOAuthGrantByToken :execrows
UPDATE oauth_grants SET revoked_at = now()
WHERE (token_hash = $1 OR refresh_hash = $1) AND client_id = $2 AND revoked_at IS NULL;

-- name: PurgeOAuthCodes :exec
DELETE FROM oauth_codes WHERE expires_at < now() - interval '1 day';
