-- +goose Up

-- Set the first time a member's session is used from the iOS or watch app, so the
-- home checklist can tick off "connect the mobile app".
ALTER TABLE users ADD COLUMN app_connected_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN app_connected_at;
