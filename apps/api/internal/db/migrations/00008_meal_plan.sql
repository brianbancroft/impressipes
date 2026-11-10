-- +goose Up

-- How often someone pressed "I cooked this". Independent of the weekly plan.
ALTER TABLE recipes
    ADD COLUMN times_cooked integer NOT NULL DEFAULT 0 CHECK (times_cooked >= 0),
    ADD COLUMN last_cooked_at timestamptz;

-- Buy this only when it is on sale. The flag is edited in item details; list rows only display it.
ALTER TABLE list_items
    ADD COLUMN on_sale_only boolean NOT NULL DEFAULT false;

-- One planned meal per slot per day. recipe_id is optional: tacos-by-memory have a title and no recipe.
CREATE TABLE meal_plan_entries (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_date    date NOT NULL,
    slot         text NOT NULL CHECK (slot IN ('breakfast', 'lunch', 'dinner', 'extra')),
    title        text NOT NULL,
    recipe_id    uuid REFERENCES recipes (id) ON DELETE SET NULL,
    completed_at timestamptz,
    completed_by uuid REFERENCES users (id),
    notes        text NOT NULL DEFAULT '',
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plan_date, slot)
);
CREATE INDEX meal_plan_entries_recipe_id_idx ON meal_plan_entries (recipe_id);

-- +goose Down

DROP TABLE meal_plan_entries;
ALTER TABLE list_items DROP COLUMN on_sale_only;
ALTER TABLE recipes DROP COLUMN last_cooked_at, DROP COLUMN times_cooked;
