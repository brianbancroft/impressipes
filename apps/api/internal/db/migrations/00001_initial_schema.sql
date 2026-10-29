-- +goose Up

CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name text NOT NULL,
    role         text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    avatar_color text NOT NULL,
    disabled_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE invites (
    token_hash text PRIMARY KEY,
    -- null for the first-run admin invite
    created_by uuid REFERENCES users (id),
    -- set for a re-invite of an existing member (lost device)
    member_id  uuid REFERENCES users (id),
    expires_at timestamptz NOT NULL,
    used_by    uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE passkeys (
    credential_id bytea PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    public_key    bytea NOT NULL,
    sign_count    bigint NOT NULL DEFAULT 0,
    transports    text[] NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX passkeys_user_id_idx ON passkeys (user_id);

CREATE TABLE sessions (
    id         text PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE oauth_clients (
    id            text PRIMARY KEY,
    name          text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE oauth_grants (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  text NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX oauth_grants_user_id_idx ON oauth_grants (user_id);

CREATE TABLE api_tokens (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text NOT NULL,
    token_hash   text NOT NULL UNIQUE,
    last_used_at timestamptz,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_tokens_user_id_idx ON api_tokens (user_id);

CREATE TABLE recipes (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title         text NOT NULL,
    summary       text NOT NULL DEFAULT '',
    servings      numeric,
    prep_minutes  integer CHECK (prep_minutes >= 0),
    cook_minutes  integer CHECK (cook_minutes >= 0),
    source_url    text,
    source_kind   text NOT NULL DEFAULT 'manual' CHECK (source_kind IN ('url', 'photo', 'manual', 'ai')),
    notes         text NOT NULL DEFAULT '',
    created_by    uuid NOT NULL REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE INDEX recipes_created_by_idx ON recipes (created_by);

CREATE TABLE recipe_favourites (
    recipe_id uuid NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind      text NOT NULL CHECK (kind IN ('favourite', 'want_to_make')),
    PRIMARY KEY (recipe_id, user_id, kind)
);

CREATE TABLE recipe_images (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id  uuid NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    position   integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX recipe_images_recipe_id_idx ON recipe_images (recipe_id);

CREATE TABLE recipe_ingredients (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id uuid NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    position  integer NOT NULL,
    section   text NOT NULL DEFAULT '',
    quantity  numeric,
    unit      text NOT NULL DEFAULT '',
    item      text NOT NULL,
    prep_note text NOT NULL DEFAULT '',
    raw_text  text NOT NULL DEFAULT ''
);
CREATE INDEX recipe_ingredients_recipe_id_idx ON recipe_ingredients (recipe_id, position);

CREATE TABLE recipe_steps (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id     uuid NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    position      integer NOT NULL,
    text          text NOT NULL,
    timer_seconds integer CHECK (timer_seconds > 0)
);
CREATE INDEX recipe_steps_recipe_id_idx ON recipe_steps (recipe_id, position);

CREATE TABLE tags (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE
);

CREATE TABLE recipe_tags (
    recipe_id uuid NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    tag_id    uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);
CREATE INDEX recipe_tags_tag_id_idx ON recipe_tags (tag_id);

CREATE TABLE lists (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    emoji      text NOT NULL DEFAULT '',
    colour     text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    archived   boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE list_categories (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id    uuid NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    name       text NOT NULL,
    colour     text NOT NULL,
    icon       text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0
);
CREATE INDEX list_categories_list_id_idx ON list_categories (list_id, sort_order);

-- Monotonic version for the sync protocol: GET /api/sync?since=<version>.
CREATE SEQUENCE list_item_version_seq;

CREATE TABLE list_items (
    -- client-generated so offline outbox retries are idempotent
    id               uuid PRIMARY KEY,
    list_id          uuid NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    category_id      uuid REFERENCES list_categories (id) ON DELETE SET NULL,
    name             text NOT NULL,
    quantity_text    text NOT NULL DEFAULT '',
    note             text NOT NULL DEFAULT '',
    checked_at       timestamptz,
    checked_by       uuid REFERENCES users (id),
    added_by         uuid NOT NULL REFERENCES users (id),
    source_recipe_id uuid REFERENCES recipes (id) ON DELETE SET NULL,
    position         integer NOT NULL DEFAULT 0,
    version          bigint NOT NULL DEFAULT nextval('list_item_version_seq'),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz
);
CREATE INDEX list_items_list_id_idx ON list_items (list_id, position);
CREATE INDEX list_items_version_idx ON list_items (version);

-- +goose StatementBegin
CREATE FUNCTION bump_list_item_version() RETURNS trigger AS $$
BEGIN
    NEW.version := nextval('list_item_version_seq');
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER list_items_bump_version
    BEFORE UPDATE ON list_items
    FOR EACH ROW EXECUTE FUNCTION bump_list_item_version();

CREATE TABLE item_memory (
    list_id          uuid NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    normalized_name  text NOT NULL,
    last_category_id uuid REFERENCES list_categories (id) ON DELETE SET NULL,
    last_quantity    text NOT NULL DEFAULT '',
    use_count        integer NOT NULL DEFAULT 1,
    PRIMARY KEY (list_id, normalized_name)
);

CREATE TABLE activity (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    uuid REFERENCES users (id),
    -- 'web', 'ios', 'watch' or 'mcp:<client name>'
    via        text NOT NULL,
    action     text NOT NULL,
    list_id    uuid REFERENCES lists (id) ON DELETE SET NULL,
    recipe_id  uuid REFERENCES recipes (id) ON DELETE SET NULL,
    detail     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_list_id_idx ON activity (list_id, created_at DESC);

-- +goose Down

DROP TABLE activity;
DROP TABLE item_memory;
DROP TABLE list_items;
DROP FUNCTION bump_list_item_version();
DROP SEQUENCE list_item_version_seq;
DROP TABLE list_categories;
DROP TABLE lists;
DROP TABLE recipe_tags;
DROP TABLE tags;
DROP TABLE recipe_steps;
DROP TABLE recipe_ingredients;
DROP TABLE recipe_images;
DROP TABLE recipe_favourites;
DROP TABLE recipes;
DROP TABLE api_tokens;
DROP TABLE oauth_grants;
DROP TABLE oauth_clients;
DROP TABLE sessions;
DROP TABLE passkeys;
DROP TABLE invites;
DROP TABLE users;
