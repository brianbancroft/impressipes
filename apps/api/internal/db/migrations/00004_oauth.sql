-- +goose Up

-- A grant is one member authorising one client. The access token is short-lived and
-- the refresh token rotates, so both live on the grant row and are replaced together.
ALTER TABLE oauth_grants
    ADD COLUMN refresh_hash       text UNIQUE,
    ADD COLUMN refresh_expires_at timestamptz,
    ADD COLUMN last_used_at       timestamptz;

-- Authorization codes are single-use and live for minutes.
CREATE TABLE oauth_codes (
    code_hash      text PRIMARY KEY,
    client_id      text NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    user_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    redirect_uri   text NOT NULL,
    code_challenge text NOT NULL,
    expires_at     timestamptz NOT NULL,
    used_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE oauth_codes;
ALTER TABLE oauth_grants
    DROP COLUMN last_used_at,
    DROP COLUMN refresh_expires_at,
    DROP COLUMN refresh_hash;
