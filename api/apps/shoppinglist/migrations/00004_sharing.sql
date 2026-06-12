-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS shoppinglist.shoppinglist_access (
    owner_user_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    can_edit BOOL NOT NULL DEFAULT TRUE,
    PRIMARY KEY (owner_user_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_shoppinglist_access_user
ON shoppinglist.shoppinglist_access (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shoppinglist.shoppinglist_access;
-- +goose StatementEnd
