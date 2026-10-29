-- +goose Up

-- Email and password are optional until a member adds them. users.email is only
-- ever set once the address has been confirmed, so a magic link can always be
-- sent to it.
ALTER TABLE users
    ADD COLUMN email         text,
    ADD COLUMN password_hash text;
CREATE UNIQUE INDEX users_email_idx ON users (email);

-- One row per emailed link + 6-digit code. 'login' signs a member in at their
-- confirmed address; 'verify' confirms a new address for a member who is already
-- signed in. Both the link token and the code are stored hashed.
CREATE TABLE login_codes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('login', 'verify')),
    email      text NOT NULL,
    link_hash  text NOT NULL UNIQUE,
    code_hash  text NOT NULL,
    attempts   int NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_codes_email_idx ON login_codes (email, created_at DESC);

-- +goose Down
DROP TABLE login_codes;
DROP INDEX users_email_idx;
ALTER TABLE users DROP COLUMN password_hash, DROP COLUMN email;
