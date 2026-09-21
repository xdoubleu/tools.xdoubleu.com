-- games.steam_achievements' only index is its primary key on
-- (name, user_id, game_id) — useless for a (game_id, user_id) predicate since
-- name leads. The FK on (game_id, user_id) referencing steam_games doesn't
-- get an index automatically either. Every GetAchievementsForGames call
-- (the games detail page, powering GET /games/:id) therefore did a full
-- sequential scan of the whole table — see issue #1716.

-- +goose Up
-- +goose StatementBegin
CREATE INDEX idx_steam_achievements_game_user
ON games.steam_achievements (game_id, user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX games.idx_steam_achievements_game_user;
-- +goose StatementEnd
