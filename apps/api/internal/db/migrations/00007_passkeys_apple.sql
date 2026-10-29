-- +goose Up

-- Real WebAuthn passkeys share the table the invite flow already writes a
-- placeholder row to (empty public_key). Everything below that lists or checks
-- passkeys ignores rows whose public_key is empty.
ALTER TABLE passkeys
    ADD COLUMN name            text NOT NULL DEFAULT '',
    ADD COLUMN backup_eligible boolean NOT NULL DEFAULT false,
    ADD COLUMN backup_state    boolean NOT NULL DEFAULT false,
    ADD COLUMN aaguid          bytea NOT NULL DEFAULT '',
    ADD COLUMN last_used_at    timestamptz;

-- A member's Apple ID, keyed by the stable "sub" Apple issues per app group. One
-- Apple ID per member, and one member per Apple ID.
CREATE TABLE apple_identities (
    sub        text PRIMARY KEY,
    user_id    uuid NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    email      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE apple_identities;
ALTER TABLE passkeys
    DROP COLUMN last_used_at, DROP COLUMN aaguid, DROP COLUMN backup_state,
    DROP COLUMN backup_eligible, DROP COLUMN name;
