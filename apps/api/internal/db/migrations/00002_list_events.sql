-- +goose Up

-- Tells the API instances, over LISTEN/NOTIFY, that a list item changed. The
-- payload is only a pointer; clients fetch the change with GET /api/sync.
-- +goose StatementBegin
CREATE FUNCTION notify_list_item_change() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('list_events', json_build_object(
        'listId', NEW.list_id, 'itemId', NEW.id, 'version', NEW.version)::text);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER list_items_notify
    AFTER INSERT OR UPDATE ON list_items
    FOR EACH ROW EXECUTE FUNCTION notify_list_item_change();

-- +goose Down

DROP TRIGGER list_items_notify ON list_items;
DROP FUNCTION notify_list_item_change();
