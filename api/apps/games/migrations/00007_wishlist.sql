-- +goose Up
-- +goose StatementBegin
CREATE TABLE games.steam_wishlist (
    game_id BIGINT NOT NULL,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    image_url TEXT NOT NULL DEFAULT '',
    priority INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, game_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE games.steam_wishlist;
-- +goose StatementEnd
