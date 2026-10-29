-- +goose Up

-- batch_id groups the rows written by one AI request, so the feed can show and
-- undo them together. Undoing records a new row (undo_of) and stamps the original
-- (undone_at); the original is never erased.
ALTER TABLE activity
    ADD COLUMN batch_id  uuid,
    ADD COLUMN undone_at timestamptz,
    ADD COLUMN undone_by uuid REFERENCES users (id),
    ADD COLUMN undo_of   bigint REFERENCES activity (id);
CREATE INDEX activity_batch_id_idx ON activity (batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX activity_created_at_idx ON activity (created_at DESC, id DESC);

-- +goose Down

DROP INDEX activity_created_at_idx;
DROP INDEX activity_batch_id_idx;
ALTER TABLE activity
    DROP COLUMN undo_of,
    DROP COLUMN undone_by,
    DROP COLUMN undone_at,
    DROP COLUMN batch_id;
